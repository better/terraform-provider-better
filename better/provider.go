package better

import (
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func Provider() *schema.Provider {
	return &schema.Provider{
		ResourcesMap: map[string]*schema.Resource{
			"better_database_password":             resourceDatabasePassword(),
			"better_database_password_association": resourceDatabasePasswordAssociation(),
		},
	}
}
