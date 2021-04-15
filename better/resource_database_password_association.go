package better

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	sdm "github.com/strongdm/strongdm-sdk-go"
)

func getDatabasePasswordId(d *schema.ResourceData) string {
	ids := []string{
		getSecretId(d),
		d.Get("db_id").(string),
	}

	return strings.Join(Compact(ids), "-")
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

func updateSdmDatabase(id string, password string, ctx context.Context) (bool, error) {
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
			postgres := r.Resource.(*sdm.Postgres)
			postgres.Password = password

			_, err := client.Resources().Update(ctx, postgres)

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
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		Schema: map[string]*schema.Schema{
			"secret_id": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "id of secret",
			},
			"db_id": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "",
				Description: "id of rds instance",
			},
			"db_users": {
				Type:        schema.TypeList,
				Description: "Set of maps that define the json key for the password, and sdm resource it is associated with",
				Required:    true,
				Elem: &schema.Schema{
					Type: schema.TypeMap,
					Elem: &schema.Schema{
						Type: schema.TypeString,
					},
				},
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
	dbId := d.Get("db_id").(string)
	dbUsers := d.Get("db_users").([]interface{})
	session := getSession()

	if p, err := getPassword(secretId, session); err != nil {
		return diag.FromErr(err)
	} else {

		for _, u := range dbUsers {

			dbUser := u.(map[string]interface{})
			key := dbUser["key"].(string)
			sdmId := dbUser["sdm_id"].(string)
			password := p.Get(key)

			if sdmId != "" {
				if _, err := updateSdmDatabase(sdmId, password, ctx); err != nil {
					return diag.FromErr(err)
				}
			}

			if dbId != "" && key == "ADMIN_PASSWORD" {
				if _, err := updateRds(dbId, password, session); err != nil {
					return diag.FromErr(err)
				}
			}
		}
	}

	d.SetId(getDatabasePasswordId(d))

	return diags
}

func resourceDatabasePasswordAssociationRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	var diags diag.Diagnostics

	d.SetId(getDatabasePasswordId(d))

	return diags
}

func resourceDatabasePasswordAssociationDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	return diags
}
