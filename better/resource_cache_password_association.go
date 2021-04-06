package better

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/elasticache"
	"github.com/hashicorp/aws-sdk-go-base/tfawserr"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	sdm "github.com/strongdm/strongdm-sdk-go"
)

const (
	CacheUpdateTimeout = 30 * time.Minute

	replicationGroupAvailableMinTimeout = 10 * time.Second
	replicationGroupAvailableDelay      = 30 * time.Second

	replicationGroupDeletedMinTimeout = 10 * time.Second
	replicationGroupDeletedDelay      = 30 * time.Second

	ReplicationGroupStatusCreating     = "creating"
	ReplicationGroupStatusAvailable    = "available"
	ReplicationGroupStatusModifying    = "modifying"
	ReplicationGroupStatusDeleting     = "deleting"
	ReplicationGroupStatusCreateFailed = "create-failed"
	ReplicationGroupStatusSnapshotting = "snapshotting"
)

func getCachePasswordId(d *schema.ResourceData) string {
	ids := []string{
		getSecretId(d),
		d.Get("replication_group_id").(string),
		d.Get("sdm_id").(string),
	}

	return strings.Join(Compact(ids), "-")
}

func updateCachePassword(cacheId string, password string, strategy string, session *session.Session) (bool, error) {
	cacheClient := elasticache.New(session)
	_, err := cacheClient.ModifyReplicationGroup(&elasticache.ModifyReplicationGroupInput{
		ReplicationGroupId:      aws.String(cacheId),
		ApplyImmediately:        aws.Bool(true),
		AuthToken:               aws.String(password),
		AuthTokenUpdateStrategy: aws.String(strategy),
	})

	if err != nil {
		return false, fmt.Errorf("error updating ElastiCache password (%s): %w", cacheId, err)
	}

	if _, err := ReplicationGroupAvailable(cacheClient, cacheId); err != nil {
		return false, fmt.Errorf("error waiting for ElastiCache Instance (%s) update: %w", cacheId, err)
	}

	return err == nil, err
}

// ReplicationGroupByID retrieves an ElastiCache Replication Group by id.
func ReplicationGroupByID(conn *elasticache.ElastiCache, id string) (*elasticache.ReplicationGroup, error) {
	input := &elasticache.DescribeReplicationGroupsInput{
		ReplicationGroupId: aws.String(id),
	}
	output, err := conn.DescribeReplicationGroups(input)
	if tfawserr.ErrCodeEquals(err, elasticache.ErrCodeReplicationGroupNotFoundFault) {
		return nil, &resource.NotFoundError{
			LastError:   err,
			LastRequest: input,
		}
	}
	if err != nil {
		return nil, err
	}

	if output == nil || len(output.ReplicationGroups) == 0 || output.ReplicationGroups[0] == nil {
		return nil, &resource.NotFoundError{
			Message:     "empty result",
			LastRequest: input,
		}
	}

	return output.ReplicationGroups[0], nil
}

// NotFound returns true if the error represents a "resource not found" condition.
// Specifically, NotFound returns true if the error or a wrapped error is of type
// resource.NotFoundError.
func NotFound(err error) bool {
	var e *resource.NotFoundError // nosemgrep: is-not-found-error
	return errors.As(err, &e)
}

func updateSdmRedis(id string, password string, ctx context.Context) (bool, error) {
	accessKey := os.Getenv("SDM_API_ACCESS_KEY")
	secretKey := os.Getenv("SDM_API_SECRET_KEY")

	if accessKey == "" || secretKey == "" {
		return false, nil
	}

	if client, err := sdm.New(accessKey, secretKey); err != nil {
		return err == nil, err
	} else {
		if r, err := client.Resources().Get(ctx, id); err != nil {
			return err == nil, err
		} else {
			redis := r.Resource.(*sdm.ElasticacheRedis)
			redis.Password = password

			_, err := client.Resources().Update(ctx, redis)

			return err == nil, err
		}
	}
}

// ReplicationGroupStatus fetches the Replication Group and its Status
func ReplicationGroupStatus(conn *elasticache.ElastiCache, replicationGroupID string) resource.StateRefreshFunc {
	return func() (interface{}, string, error) {
		rg, err := ReplicationGroupByID(conn, replicationGroupID)
		if NotFound(err) {
			return nil, "", nil
		}
		if err != nil {
			return nil, "", err
		}

		return rg, aws.StringValue(rg.Status), nil
	}
}

// ReplicationGroupAvailable waits for a ReplicationGroup to return Available
func ReplicationGroupAvailable(conn *elasticache.ElastiCache, replicationGroupID string) (*elasticache.ReplicationGroup, error) {
	stateConf := &resource.StateChangeConf{
		Pending: []string{
			ReplicationGroupStatusCreating,
			ReplicationGroupStatusModifying,
			ReplicationGroupStatusSnapshotting,
		},
		Target:     []string{ReplicationGroupStatusAvailable},
		Refresh:    ReplicationGroupStatus(conn, replicationGroupID),
		Timeout:    CacheUpdateTimeout,
		MinTimeout: replicationGroupAvailableMinTimeout,
		Delay:      replicationGroupAvailableDelay,
	}

	outputRaw, err := stateConf.WaitForState()
	if v, ok := outputRaw.(*elasticache.ReplicationGroup); ok {
		return v, err
	}
	return nil, err
}

func resourceCachePasswordAssociation() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceCachePasswordAssociationCreate,
		ReadContext:   resourceCachePasswordAssociationRead,
		UpdateContext: resourceCachePasswordAssociationRead,
		DeleteContext: resourceCachePasswordAssociationDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		Schema: map[string]*schema.Schema{
			"secret_id": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "id of secret",
			},
			"replication_group_id": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "",
				Description: "id of the ElastiCache replication group",
			},
			"sdm_id": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "",
				Description: "id of sdm resource",
			},
		},
		Timeouts: &schema.ResourceTimeout{
			Default: schema.DefaultTimeout(60 * time.Second),
		},
	}
}

func resourceCachePasswordAssociationCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	var diags diag.Diagnostics

	secretId := getSecretId(d)
	cacheId := d.Get("replication_group_id").(string)
	// sdmId := d.Get("sdm_id").(string)
	session := getSession()

	if p, err := getPassword(secretId, session); err != nil {
		return diag.FromErr(err)
	} else {

		password := p.Get("AUTH_TOKEN")

		if cacheId != "" {
			if _, err := updateCachePassword(cacheId, password, "ROTATE", session); err != nil {
				return diag.FromErr(err)
			}
			if _, err := updateCachePassword(cacheId, password, "SET", session); err != nil {
				return diag.FromErr(err)
			}

			// if sdmId != "" {
			// 	if _, err := updateSdmRedis(sdmId, password, ctx); err != nil {
			// 		return diag.FromErr(err)
			// 	}
			// }
		}
	}

	d.SetId(getCachePasswordId(d))

	return diags
}

func resourceCachePasswordAssociationRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	var diags diag.Diagnostics

	d.SetId(getCachePasswordId(d))

	return diags
}

func resourceCachePasswordAssociationDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	return diags
}
