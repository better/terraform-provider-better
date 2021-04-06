package better

import (
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type Password struct {
	AdminPassword        string `json:"ADMIN_PASSWORD,omitempty"`
	AuthToken            string `json:"AUTH_TOKEN,omitempty"`
	ReadOnlyUserPassword string `json:"READONLY_USER_PASSWORD,omitempty"`
	UserPassword         string `json:"USER_PASSWORD,omitempty"`
}

func (p *Password) Get(key string) string {
	switch key {
	case "ADMIN_PASSWORD":
		return p.AdminPassword
	case "AUTH_TOKEN":
		return p.AuthToken
	case "READONLY_USER_PASSWORD":
		return p.ReadOnlyUserPassword
	case "USER_PASSWORD":
		return p.UserPassword
	}

	return p.ReadOnlyUserPassword
}

func Compact(d []string) []string {
	r := make([]string, 0)

	for _, v := range d {
		if v != "" {
			r = append(r, v)
		}
	}

	return r
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
