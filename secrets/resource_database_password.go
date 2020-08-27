package secrets

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"time"
)

type Password struct {
	StagingAdminPassword           string `json:"STAGING_ADMIN_PASSWORD"`
	StagingUserPassword            string `json:"STAGING_USER_PASSWORD"`
	StagingReadOnlyUserPassword    string `json:"STAGING_READONLY_USER_PASSWORD"`
	ProductionAdminPassword        string `json:"PRODUCTION_ADMIN_PASSWORD"`
	ProductionUserPassword         string `json:"PRODUCTION_USER_PASSWORD"`
	ProductionReadOnlyUserPassword string `json:"PRODUCTION_READONLY_USER_PASSWORD"`
}

func getSession() *session.Session {
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String("us-east-1")},
	)

	if err == nil {
		return sess
	} else {
		fmt.Println(err.Error())
		return nil
	}
}

func getSecretId(d *schema.ResourceData) string {
	return d.Get("secret_id").(string)
}

func generateRandomPassword(svc *secretsmanager.SecretsManager) string {
	gpi := &secretsmanager.GetRandomPasswordInput{
		ExcludePunctuation: aws.Bool(true),
		PasswordLength:     aws.Int64(32),
	}

	gpo, err := svc.GetRandomPassword(gpi)

	if err != nil {
		fmt.Println(err.Error())
		return "FAILED_TO_GENERATE_RANDOM_PASSWORD"
	}

	return *gpo.RandomPassword
}

func resourceDatabasePassword() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceDatabasePasswordCreate,
		ReadContext:   resourceDatabasePasswordRead,
		UpdateContext: resourceDatabasePasswordRead,
		DeleteContext: resourceDatabasePasswordDelete,
		Schema: map[string]*schema.Schema{
			"secret_id": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "id of secret",
			},
		},
		Timeouts: &schema.ResourceTimeout{
			Default: schema.DefaultTimeout(60 * time.Second),
		},
	}
}

func resourceDatabasePasswordCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	var diags diag.Diagnostics

	svc := secretsmanager.New(getSession())

	secret := Password{
		StagingAdminPassword:           generateRandomPassword(svc),
		StagingUserPassword:            generateRandomPassword(svc),
		StagingReadOnlyUserPassword:    generateRandomPassword(svc),
		ProductionAdminPassword:        generateRandomPassword(svc),
		ProductionUserPassword:         generateRandomPassword(svc),
		ProductionReadOnlyUserPassword: generateRandomPassword(svc),
	}

	secretString, err := json.Marshal(secret)

	if err != nil {
		return diag.FromErr(err)
	}

	secretId := getSecretId(d)

	psvi := &secretsmanager.PutSecretValueInput{
		SecretId:     aws.String(secretId),
		SecretString: aws.String(string(secretString)),
	}

	_, err = svc.PutSecretValue(psvi)

	if err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("secret_id", secretId); err != nil {
		return diag.FromErr(err)
	}

	d.SetId(secretId)

	return diags
}

func resourceDatabasePasswordRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	var diags diag.Diagnostics

	svc := secretsmanager.New(getSession())

	gsvi := &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(getSecretId(d)),
	}

	gsvo, err := svc.GetSecretValue(gsvi)

	if err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("secret_id", gsvo.ARN); err != nil {
		return diag.FromErr(err)
	}

	d.SetId(*gsvo.ARN)

	return diags
}

func resourceDatabasePasswordDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	return diags
}
