package better

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/mq"
	"github.com/hashicorp/aws-sdk-go-base/tfawserr"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	sdm "github.com/strongdm/strongdm-sdk-go"
)

const (
	BrokerRebootTimeout = 30 * time.Minute
)

func getMqPasswordId(d *schema.ResourceData) string {
	ids := []string{
		getSecretId(d),
		d.Get("mq_id").(string),
		d.Get("sdm_id").(string),
	}

	return strings.Join(Compact(ids), "-")
}

func updateMq(id string, user string, password string, consoleAccess bool, session *session.Session) (bool, error) {
	mqClient := mq.New(session)

	_, err := mqClient.UpdateUser(&mq.UpdateUserRequest{
		BrokerId:      aws.String(id),
		Username:      aws.String(user),
		ConsoleAccess: aws.Bool(consoleAccess),
		Password:      aws.String(password),
	})

	return err == nil, err
}

func rebootMq(mqId string, session *session.Session) (bool, error) {
	mqClient := mq.New(session)

	_, err := mqClient.RebootBroker(&mq.RebootBrokerInput{
		BrokerId: aws.String(mqId),
	})
	if err != nil {
		return false, fmt.Errorf("error rebooting MQ Broker (%s): %w", mqId, err)
	}

	if _, err := BrokerRebooted(mqClient, mqId); err != nil {
		return false, fmt.Errorf("error waiting for MQ Broker (%s) reboot: %w", mqId, err)
	}

	return err == nil, err
}

func BrokerStatus(conn *mq.MQ, id string) resource.StateRefreshFunc {
	return func() (interface{}, string, error) {
		output, err := conn.DescribeBroker(&mq.DescribeBrokerInput{
			BrokerId: aws.String(id),
		})

		if tfawserr.ErrCodeEquals(err, mq.ErrCodeNotFoundException) {
			return nil, "", nil
		}

		if err != nil {
			return nil, "", err
		}

		if output == nil {
			return nil, "", nil
		}

		return output, aws.StringValue(output.BrokerState), nil
	}
}

func BrokerRebooted(conn *mq.MQ, id string) (*mq.DescribeBrokerResponse, error) {
	stateConf := resource.StateChangeConf{
		Pending: []string{
			mq.BrokerStateRebootInProgress,
		},
		Target:  []string{mq.BrokerStateRunning},
		Timeout: BrokerRebootTimeout,
		Refresh: BrokerStatus(conn, id),
	}
	outputRaw, err := stateConf.WaitForState()

	if output, ok := outputRaw.(*mq.DescribeBrokerResponse); ok {
		return output, err
	}

	return nil, err
}

func updateSdmMq(id string, user string, password string, ctx context.Context) (bool, error) {
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
			website := r.Resource.(*sdm.HTTPBasicAuth)
			website.Username = user
			website.Password = password

			_, err := client.Resources().Update(ctx, website)

			return err == nil, err
		}
	}
}

func resourceMqPasswordAssociation() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceMqPasswordAssociationCreate,
		ReadContext:   resourceMqPasswordAssociationRead,
		UpdateContext: resourceMqPasswordAssociationRead,
		DeleteContext: resourceMqPasswordAssociationDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		Schema: map[string]*schema.Schema{
			"secret_id": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "id of secret",
			},
			"mq_id": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "",
				Description: "id of the MQ broker",
			},
			"mq_users": {
				Type:        schema.TypeList,
				Description: "Set of maps that define username, console access, and the json key for the password",
				Required:    true,
				Elem: &schema.Schema{
					Type: schema.TypeMap,
					Elem: &schema.Schema{
						Type: schema.TypeString,
					},
				},
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

func resourceMqPasswordAssociationCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	var diags diag.Diagnostics

	secretId := getSecretId(d)
	mqId := d.Get("mq_id").(string)
	sdmId := d.Get("sdm_id").(string)
	mqUsers := d.Get("mq_users").([]interface{})
	session := getSession()

	if p, err := getPassword(secretId, session); err != nil {
		return diag.FromErr(err)
	} else {

		for _, u := range mqUsers {

			mqUser := u.(map[string]interface{})
			user := mqUser["user"].(string)
			consoleAccess, _ := strconv.ParseBool(mqUser["console_access"].(string))
			key := mqUser["key"].(string)
			password := p.Get(key)

			if mqId != "" && user != "" {
				if _, err := updateMq(mqId, user, password, consoleAccess, session); err != nil {
					return diag.FromErr(err)
				}

				if sdmId != "" && consoleAccess {
					if _, err := updateSdmMq(sdmId, user, password, ctx); err != nil {
						return diag.FromErr(err)
					}
				}
			}
		}

		// Reboot MQ broker to apply the changes
		if mqId != "" {
			if _, err := rebootMq(mqId, session); err != nil {
				return diag.FromErr(err)
			}
		}
	}

	d.SetId(getMqPasswordId(d))

	return diags
}

func resourceMqPasswordAssociationRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	var diags diag.Diagnostics

	d.SetId(getMqPasswordId(d))

	return diags
}

func resourceMqPasswordAssociationDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	return diags
}
