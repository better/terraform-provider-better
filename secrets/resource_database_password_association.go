package secrets

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"time"
)

func getId(secretId string, key string, rdsDbId string) string {
	return fmt.Sprintf("%s-%s-%s", secretId, key, rdsDbId)
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
			"key": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "json key in secret to associate",
			},
			"rds_db_id": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "id of rds instance",
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
	key := d.Get("key").(string)
	rdsDbId := d.Get("rds_db_id").(string)
	session := getSession()
	password := Password{}

	secretsManagerClient := secretsmanager.New(session)
	rdsClient := rds.New(session)

	gsvi := &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(secretId),
	}

	if gsvo, err := secretsManagerClient.GetSecretValue(gsvi); err != nil {
		return diag.FromErr(err)
	} else if err := json.Unmarshal([]byte(*gsvo.SecretString), &password); err != nil {
		return diag.FromErr(err)
	}

	mdbii := &rds.ModifyDBInstanceInput{
		DBInstanceIdentifier: aws.String(rdsDbId),
		MasterUserPassword:   aws.String(password.ProductionAdminPassword),
		ApplyImmediately:     aws.Bool(true),
	}

	if _, err := rdsClient.ModifyDBInstance(mdbii); err != nil {
		return diag.FromErr(err)
	}

	d.SetId(getId(secretId, key, rdsDbId))

	return diags
}

func resourceDatabasePasswordAssociationRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	var diags diag.Diagnostics

	secretId := getSecretId(d)
	key := d.Get("key").(string)
	rdsDbId := d.Get("rds_db_id").(string)

	d.SetId(getId(secretId, key, rdsDbId))

	return diags
}

func resourceDatabasePasswordAssociationDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	return diags
}
