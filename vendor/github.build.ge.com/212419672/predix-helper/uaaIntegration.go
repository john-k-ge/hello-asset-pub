package uaaIntegration

import (
	"log"
	"net/http"

	"os"

	"strings"

	"errors"

	"github.com/cloudfoundry-community/go-cfenv"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

type UaaClient struct {
	Name         string   `json:"name"`
	Id           string   `json:"client_id"`
	Secret       string   `json:"client_secret"`
	Grants       []string `json:"authorized_grant_types"`
	Scopes       []string `json:"scope"`
	Authorities  []string `json:"authorities"`
	Autoapproves []string `json:"autoapprove"`
}

type PredixUaaCreds struct {
	ClientId     string
	ClientSecret string
	Uri          string
	Scopes       []string
}

var commonScopesAndAuthorities = []string{
	"scim.me",
	"uaa.resource",
	"openid",
}

var adminScopesAndAuthorities = []string{
	"clients.read",
	"zones.read",
	"clients.secret",
	"idps.write",
	"uaa.resource",
	//"zones.5b9036c2-4040-41c0-b247-aa78e292ffe2.admin",
	"clients.write",
	"clients.admin",
	"uaa.admin",
	"idps.read",
	"scim.write",
	"scim.read",
}

var commonGrants = []string{
	"refresh_token",
	"client_credentials",
	"password",
	"authorization_code",
}

var implicitUaaConfig = &oauth2.Config{
	Scopes:   []string{""},
	ClientID: "cf",
	Endpoint: oauth2.Endpoint{
		//AuthURL:  "https://uaa.system.aws-usw02-pr.ice.predix.io/oauth/authorize",
		//TokenURL: "https://uaa.system.aws-usw02-pr.ice.predix.io/oauth/token",
		AuthURL:  "https://" + os.Getenv("UAA") + "/oauth/authorize",
		TokenURL: "https://" + os.Getenv("UAA") + "/oauth/token",
	},
}

func GetSimpleClientConfig(id, secret string, scopes []string) *UaaClient {
	return &UaaClient{
		Name:         id,
		Id:           id,
		Secret:       secret,
		Grants:       commonGrants,
		Scopes:       append(commonScopesAndAuthorities, scopes...),
		Authorities:  append(commonScopesAndAuthorities, scopes...),
		Autoapproves: append(commonScopesAndAuthorities, scopes...),
	}
}

func getGenericUaaClient(uaaConf *PredixUaaCreds) (*http.Client, error) {
	var dummy *http.Client

	tempUaaConfig := &clientcredentials.Config{
		ClientID:     uaaConf.ClientId,
		ClientSecret: uaaConf.ClientSecret,
		Scopes:       uaaConf.Scopes,
		TokenURL:     uaaConf.Uri + "/oauth/token",
	}

	_, err := tempUaaConfig.Token(oauth2.NoContext)
	if err != nil {
		log.Printf("Could not get token: %v", err)
		return dummy, errors.New("Could not get token: " + err.Error())
	}

	return tempUaaConfig.Client(oauth2.NoContext), nil
}

func GetAdminUaaClient(uaaConf *PredixUaaCreds) (*http.Client, error) {
	//var dummy *http.Client
	for _, scope := range adminScopesAndAuthorities {
		uaaConf.Scopes = append(uaaConf.Scopes, scope)
	}
	//tempUaaConfig := &clientcredentials.Config{
	//	ClientID:     uaaConf.ClientId,
	//	ClientSecret: uaaConf.ClientSecret,
	//	Scopes:       uaaConf.Scopes,
	//	TokenURL:     uaaConf.Uri + "/oauth/token",
	//}
	//
	//_, err := tempUaaConfig.Token(oauth2.NoContext)
	//if err != nil {
	//	log.Printf("Could not get token: %v", err)
	//	return dummy, errors.New("Could not get token: " + err.Error())
	//}

	return getGenericUaaClient(uaaConf)
}

func GetPredixUaaClient(uaaConf *PredixUaaCreds) (*http.Client, error) {
	var dummy *http.Client
	for _, scope := range commonScopesAndAuthorities {
		uaaConf.Scopes = append(uaaConf.Scopes, scope)
	}
	tempUaaConfig := &clientcredentials.Config{
		ClientID:     uaaConf.ClientId,
		ClientSecret: uaaConf.ClientSecret,
		Scopes:       uaaConf.Scopes,
		TokenURL:     uaaConf.Uri + "/oauth/token",
	}

	_, err := tempUaaConfig.Token(oauth2.NoContext)
	if err != nil {
		log.Printf("Could not get token: %v", err)
		return dummy, errors.New("Could not get token: " + err.Error())
	}

	return tempUaaConfig.Client(oauth2.NoContext), nil
}

func GetPlatformUaaClient() *http.Client {
	appEnv, _ := cfenv.Current()
	services := appEnv.Services
	cups, err := services.WithName("uaa-integration")

	var env_uid, env_pass string

	for credKey, credVal := range cups.Credentials {
		switch {
		case strings.EqualFold(credKey, "USERNAME"):
			env_uid = credVal.(string)

		case strings.EqualFold(credKey, "PASS"):
			env_pass = credVal.(string)
		}
	}

	token, err := implicitUaaConfig.PasswordCredentialsToken(oauth2.NoContext, env_uid, env_pass)
	if err != nil {
		log.Printf("Could not get token: %v", err)
		return nil
	}

	return implicitUaaConfig.Client(oauth2.NoContext, token)
}
