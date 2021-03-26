package better

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/mq"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	sdm "github.com/strongdm/strongdm-sdk-go"
)

func getMqPasswordId(d *schema.ResourceData) string {
	ids := []string{
		getSecretId(d),
		d.Get("mq_id").(string),
		d.Get("mq_user").(string),
		d.Get("key").(string),
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
			"key": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "",
				Description: "json key for admin password to use",
			},
			"mq_id": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "",
				Description: "id of MQ broker",
			},
			"mq_user": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "admin",
				Description: "name of the admin user",
			},
			"mq_user_console_access": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     true,
				Description: "allow console access for the admin user",
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
	mqUser := d.Get("mq_user").(string)
	mqUserConsoleAccess := d.Get("mq_user_console_access").(bool)
	key := d.Get("key").(string)
	sdmId := d.Get("sdm_id").(string)
	session := getSession()

	if p, err := getPassword(secretId, session); err != nil {
		return diag.FromErr(err)
	} else {
		password := p.Get(key)

		if mqId != "" && mqUser != "" {
			if _, err := updateMq(mqId, mqUser, password, mqUserConsoleAccess, session); err != nil {
				return diag.FromErr(err)
			}
		}

		if sdmId != "" && mqUserConsoleAccess {
			if _, err := updateSdmMq(sdmId, mqUser, password, ctx); err != nil {
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
