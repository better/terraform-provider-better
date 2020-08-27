package secrets

import (
	"context"
	"encoding/json"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	sdm "github.com/strongdm/strongdm-sdk-go"
	"os"
	"strings"
	"time"
)

func getId(d *schema.ResourceData) string {
	ids := []string{
		getSecretId(d),
		d.Get("rds_db_id").(string),
		d.Get("sdm_resource_id").(string),
	}

	return strings.Join(Compact(ids), "-")
}

func getPassword(secretId string, session *session.Session) (Password, error) {
	secretsManagerClient := secretsmanager.New(session)
	password := Password{}

	gsvi := &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(secretId),
	}

	if gsvo, err := secretsManagerClient.GetSecretValue(gsvi); err != nil {
		return password, err
	} else if err := json.Unmarshal([]byte(*gsvo.SecretString), &password); err != nil {
		return password, err
	}

	return password, nil
}

func updateRds(id string, password string, session *session.Session) (bool, error) {
	rdsClient := rds.New(session)

	_, err := rdsClient.ModifyDBInstance(&rds.ModifyDBInstanceInput{
		DBInstanceIdentifier: aws.String(id),
		MasterUserPassword:   aws.String(password),
		ApplyImmediately:     aws.Bool(true),
	})

	return err == nil, err
}

func updateSdm(id string, password string) (bool, error) {
	accessKey := os.Getenv("SDM_API_ACCESS_KEY")
	secretKey := os.Getenv("SDM_API_SECRET_KEY")

	if accessKey == "" || secretKey == "" {
		return false, nil
	}

	if client, err := sdm.New(accessKey, secretKey); err != nil {
		return err == nil, err
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if r, err := client.Resources().Get(ctx, id); err != nil {
			return err == nil, err
		} else {
			var resource interface{} = r.Resource
			postgres := resource.(sdm.Postgres)
			postgres.Password = password

			_, err := client.Resources().Update(ctx, &postgres)

			return err == nil, err
		}
	}
}

func resourceDatabasePasswordAssociation() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceDatabasePasswordAssociationCreate,
		ReadContext:   resourceDatabasePasswordAssociationRead,
		UpdateContext: resourceDatabasePasswordAssociationRead,
		DeleteContext: resourceDatabasePasswordAssociationDelete,
		Schema: map[string]*schema.Schema{
			"secret_id": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "id of secret",
			},
			"rds_db_id": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "",
				Description: "id of rds instance",
			},
			"sdm_resource_id": {
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

func resourceDatabasePasswordAssociationCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	var diags diag.Diagnostics

	secretId := getSecretId(d)
	rdsDbId := d.Get("rds_db_id").(string)
	sdmResourceId := d.Get("sdm_resource_id").(string)
	session := getSession()

	if password, err := getPassword(secretId, session); err != nil {
		return diag.FromErr(err)
	} else {
		if rdsDbId != "" {
			if _, err := updateRds(rdsDbId, password.AdminPassword, session); err != nil {
				return diag.FromErr(err)
			}
		}

		if sdmResourceId != "" {
			if _, err := updateSdm(sdmResourceId, password.AdminPassword); err != nil {
				return diag.FromErr(err)
			}
		}
	}

	d.SetId(getId(d))

	return diags
}

func resourceDatabasePasswordAssociationRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	var diags diag.Diagnostics

	d.SetId(getId(d))

	return diags
}

func resourceDatabasePasswordAssociationDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	return diags
}
