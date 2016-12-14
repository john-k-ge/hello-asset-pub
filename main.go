package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"hello-asset/envStructs"

	"github.build.ge.com/212419672/cf-service-tester/cfServiceDiscovery"
	"github.build.ge.com/212419672/predix-helper"

	"github.com/cloudfoundry-community/go-cfenv"
)

var myService cfServiceDiscovery.ServiceDescriptor
var assetConfig envStructs.AssetConfig               // stores data to pass to Asset
var assetUaa, adminUaa uaaIntegration.PredixUaaCreds // stores data to get UAA token
var sampleAssetData *envStructs.AssetModel
var createUaaClientPath = "/oauth/clients"
var deleteUaaClientPath = "/oauth/clients/%s"

const assetRoot = "/assets"
const uaaLabel string = "predix-uaa"

/**

Tasks:
	1) get UAA token:
		a. Get clientID
		b. Get client secret
		c. get UAA url
		d. get scopes
		e. populate uaaConf
	2) post/get Asset
		a. get asset url
		b. get header string
		c. get header val
		d. get scopes
		e. populate assetCreds

*/

func postAsset(client *http.Client, asset *envStructs.AssetModel) error {
	payload := []*envStructs.AssetModel{
		asset,
	}

	rawPayload, err := json.Marshal(payload)
	if err != nil {
		fmt.Printf("Cannot marshal AssetModel: %v", err)
		return errors.New("Cannot marshal AssetModel: " + err.Error())
	}

	fmt.Printf("POST payload: %v\n", string(rawPayload))

	req, err := http.NewRequest("POST", assetConfig.Url+assetRoot, strings.NewReader(string(rawPayload)))
	if err != nil {
		fmt.Printf("Could not create Asset POST request: %v", err)
		return err
	}
	req.Header.Add(assetConfig.HeaderName, assetConfig.HeaderVal)
	req.Header.Add("content-type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Failed to POST to asset: %v", err)
		return err
	}

	fmt.Printf("POST received: %v\n", strconv.Itoa(resp.StatusCode))
	return nil
}

func getAsset(client *http.Client) ([]envStructs.AssetModel, error) {
	req, err := http.NewRequest("GET", assetConfig.Url+assetRoot, strings.NewReader(""))
	if err != nil {
		fmt.Printf("Could not create Asset GET request: %v", err)
		return nil, err
	}
	req.Header.Add(assetConfig.HeaderName, assetConfig.HeaderVal)
	req.Header.Add("content-type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Failed to GET to asset: %v", err)
		return nil, err
	}
	fmt.Printf("GET received: http %v\n", strconv.Itoa(resp.StatusCode))
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Failed to read response body: %v", err)
		return nil, err
	}

	fmt.Printf("Body dump: %v\n", string(body[:]))

	var assetResp []envStructs.AssetModel

	err = json.Unmarshal(body, &assetResp)
	if err != nil {
		fmt.Printf("Failed to unmarshall Asset response body: %v\n", err)
		return nil, err
	}

	return assetResp, nil
}

func exerciseAsset(w http.ResponseWriter, req *http.Request) {
	if len(assetConfig.Url) == 0 {
		fmt.Fprint(w, "I'm not bound to an Asset Service!  Please bind me!\n")
		return
	}
	fmt.Printf("In exerciseAsset()...\n")
	fmt.Printf("About to delete UAA with: %v, %v, %v\n", adminUaa.ClientId, adminUaa.ClientSecret, adminUaa.Uri)
	err := deleteUaaClient()
	if err != nil {
		fmt.Fprintf(w, "Whoops... could not clean up Uaa client: %v\n", err.Error())
		return
	}
	fmt.Printf("About to create UAA client\n")
	err = createUaaClient()
	if err != nil {
		fmt.Printf("In simpleUaaTest(), Could not create Uaa client: %v\n", err.Error())
		fmt.Fprintf(w, "Whoops... could not create the Uaa client: %v\n", err.Error())
		return
	}
	fmt.Printf("about to post to asset...\n")

	client, err := uaaIntegration.GetPredixUaaClient(&assetUaa)
	if err != nil {
		fmt.Printf("Cannot generate HttpClient: %v", err)
		fmt.Fprintf(w, "Cannot generate HttpClient: %v", err.Error())
		return
	}

	err = postAsset(client, sampleAssetData)
	if err != nil {
		fmt.Printf("Failed to post to Asset: %v", err)
		fmt.Fprintf(w, "Failed to post to Asset: %v", err.Error())
		return
	}
	fmt.Print("I POSTed an asset to the Asset Service!\n")
	fmt.Fprint(w, "I POSTed an asset to the Asset Service!\n")

	assetRespArray, err := getAsset(client)
	if err != nil {
		fmt.Printf("Failed to get Asset: %v", err)
		fmt.Fprintf(w, "Failed to get Asset: %v", err.Error())
		return
	}
	fmt.Printf("I got %v assets in the response\n", len(assetRespArray))
	fmt.Fprintf(w, "I queried Asset and got %v assets in the response\n", len(assetRespArray))

	if len(assetRespArray) == 0 {
		fmt.Fprint(w, "Oh oh... I should have gotten something back.  Something is wrong!\n")
		return
	}

	for _, asset := range assetRespArray {
		fmt.Print("Asset summary:\n")
		fmt.Printf("ID: %v\n", asset.Id)
		fmt.Printf("Serial: %v\n", asset.Serial)
		if sampleAssetData.Id == asset.Id {
			fmt.Printf("Match!  %v ID equals %v ID", sampleAssetData.Id, asset.Id)
			fmt.Fprintf(w, "We have a match:  inserted: %v, queried: %v.  :thumbsup:\n", sampleAssetData.Id, asset.Id)
			break
		}
	}

}

func createUaaClient() error {
	fmt.Println("Starting createUaaClient()")
	adminHttpClient, err := uaaIntegration.GetAdminUaaClient(&adminUaa)
	if err != nil {
		fmt.Printf("Failed to get admin UAA client: %v\n", err)
		return errors.New("Failed to get admin UAA client: " + err.Error())
	}

	newOauthClient := uaaIntegration.GetSimpleClientConfig(assetUaa.ClientId, assetUaa.ClientSecret, assetUaa.Scopes)

	b, err := json.Marshal(newOauthClient)
	if err != nil {
		fmt.Printf("Failed to marshall the new client config: %v\n", err)
		return err
	}

	fmt.Printf("Going to create this new client: %s\n", string(b))
	req, err := http.NewRequest("POST", adminUaa.Uri+createUaaClientPath, strings.NewReader(string(b)))
	if err != nil {
		fmt.Printf("Failed to create the POST request to UAA: %v\n", err)
		return err
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "application/json")

	resp, err := adminHttpClient.Do(req)
	if err != nil {
		fmt.Printf("Failed to create the new client via POST: %v\n", err)
		return err
	}
	defer resp.Body.Close()

	fmt.Printf("Response code: %v\n", resp.StatusCode)
	theBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Could not read response from Uaa: %v\n", err.Error())
		return errors.New("Could not read response from Uaa: " + err.Error())
	}
	fmt.Printf("Uaa returned: %v\n", string(theBody))

	if resp.StatusCode == 201 {
		return nil
	}

	return errors.New("Did not receive 201 response when creating new UAA client: " + strconv.Itoa(resp.StatusCode))
}

func deleteUaaClient() error {
	adminHttpClient, err := uaaIntegration.GetAdminUaaClient(&adminUaa)
	if err != nil {
		fmt.Printf("Failed to get admin UAA client: %v", err)
		return errors.New("Failed to get admin UAA client: " + err.Error())
	}

	fmt.Print("Got an admin client!  About to build DELETE request...\n")
	finalPath := fmt.Sprintf(adminUaa.Uri+deleteUaaClientPath, assetUaa.ClientId)
	fmt.Printf("Here's the UAA path to delete:  %v\n", finalPath)

	request, err := http.NewRequest("DELETE", finalPath, strings.NewReader(""))

	if err != nil {
		log.Printf("Could not create Http request: %v\n", err)
		return errors.New("Could not create Http request: " + err.Error())
	}
	resp, err := adminHttpClient.Do(request)

	if err != nil {
		log.Printf("FAILED: %v", err)
		return errors.New("Failed to perform http DELETE: " + err.Error())

	}
	defer resp.Body.Close()

	fmt.Printf("Response code: %v\n", resp.StatusCode)
	if resp.StatusCode == 200 {
		fmt.Printf("Successfully deleted Uaa Client %v\n", assetUaa.ClientId)
		return nil
	}

	if resp.StatusCode == 404 {
		fmt.Printf("Uaa Client %v didn't exist\n", assetUaa.ClientId)
		return nil
	}

	if resp.StatusCode == 500 {
		fmt.Printf("Got 500 from UAA:  %v\n", resp.StatusCode)
		theBody, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			fmt.Printf("Could not read response from Uaa: %v\n", err.Error())
			return errors.New("Could not read response from Uaa: " + err.Error())
		}
		fmt.Printf("Uaa returned 500: %v\n", string(theBody))
		return nil
	}

	return errors.New("Did not receive 200 or 404 response when deleting existing UAA client: " + strconv.Itoa(resp.StatusCode))
}

// Return my service descriptor metadata
func serviceDescriptor(w http.ResponseWriter, req *http.Request) {
	data, err := json.Marshal(&myService)
	if err != nil {
		fmt.Printf("Cannot generate service descriptor: %v", err)
		fmt.Fprintf(w, "Cannot generate service descriptor: %v", err)
		return
	}
	fmt.Printf("Here's the data:  %s", data)
	json.NewEncoder(w).Encode(myService)
}

func init() {
	appEnv, _ := cfenv.Current()

	myService = cfServiceDiscovery.ServiceDescriptor{
		AppName:     appEnv.Name,
		AppUri:      appEnv.ApplicationURIs[0],
		ServiceName: os.Getenv("SERVICE_NAME"),
		PlanName:    os.Getenv("SERVICE_PLAN"),
	}

	assetUaa = uaaIntegration.PredixUaaCreds{
		ClientId:     os.Getenv("CLIENT"),
		ClientSecret: os.Getenv("SECRET"),
	}

	adminUaa = uaaIntegration.PredixUaaCreds{
		ClientId:     "admin",
		ClientSecret: os.Getenv("SUPERSECRET"),
	}

	services := appEnv.Services
	if len(services) > 0 {
		fmt.Printf("Asset ServiceTag = %v\n", myService.ServiceName)
		fmt.Printf("UAA ServiceTag = %v\n", uaaLabel)
		assetSvcs, _ := services.WithLabel(myService.ServiceName)

		if len(assetSvcs) > 0 {
			assetCreds := assetSvcs[0].Credentials["zone"].(map[string]interface{})

			if len(assetCreds) <= 0 {
				panic("Asset creds are null?!?")
			}

			assetConfig.Scope = assetCreds["oauth-scope"].(string)
			assetConfig.HeaderName = assetCreds["http-header-name"].(string)
			assetConfig.HeaderVal = assetCreds["http-header-value"].(string)

			assetConfig.Url = assetSvcs[0].Credentials["uri"].(string)
			assetUaa.Scopes = append(assetUaa.Scopes, assetConfig.Scope)
		}

		uaaSvcs, _ := services.WithLabel(uaaLabel)

		if len(uaaSvcs) > 0 {
			myService.TrustedIssuer = uaaSvcs[0].Credentials["issuerId"].(string)

			assetUaa.Uri = uaaSvcs[0].Credentials["uri"].(string)
			adminUaa.Uri = uaaSvcs[0].Credentials["uri"].(string)
			adminUaa.Scopes = []string{"zones." + uaaSvcs[0].Credentials["subdomain"].(string) + ".admin"}
		}

		temp := envStructs.AssetModel{
			Id:          "simpleId",
			Description: "Simple Asset",
			Uri:         assetRoot + "/simple",
			Serial:      "simple_serial",
		}
		sampleAssetData = &temp
	}

}

func main() {
	fmt.Println("Starting...")
	port := os.Getenv("PORT")
	log.Printf("Listening on port %v", port)
	if len(port) == 0 {
		port = "9000"
	}

	http.HandleFunc("/info", serviceDescriptor)
	http.HandleFunc("/ping", exerciseAsset)
	err := http.ListenAndServe(":"+port, nil)
	if err != nil {
		log.Printf("ListenAndServe: %v", err)
	}
}
