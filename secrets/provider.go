package secrets

import (
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func Provider() *schema.Provider {
	return &schema.Provider{
		ResourcesMap: map[string]*schema.Resource{
			"secrets_database_password":             resourceDatabasePassword(),
			"secrets_database_password_association": resourceDatabasePasswordAssociation(),
		},
	}
}
