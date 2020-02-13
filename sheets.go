package sheetsproxy

import (
	secretmanager "cloud.google.com/go/secretmanager/apiv1beta1"
	"context"
	"encoding/json"
	"fmt"
	"github.com/jakubincloud/sheetsproxy/sheetsproxy/util"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	goauth "golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sheets/v4"
	secretmanagerpb "google.golang.org/genproto/googleapis/cloud/secretmanager/v1beta1"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
)

var scopes = []string{
	"https://www.googleapis.com/auth/cloud-platform",
	sheets.DriveFileScope,
	sheets.DriveScope,
	sheets.SpreadsheetsScope,
}
var secretName string
var client *http.Client

func init() {
	secretName = os.Getenv("SECRET")
	//ctx := context.Background()
	//if err:= setUpClient(ctx); err != nil {
	//	log.Printf("WARNING. Client not authenticated at init time.")
	//}
}

func setUpClient(ctx context.Context) error {
	log.Println("setting up a client")

	c, err := getClient(ctx, secretName)
	if err != nil {
		log.Printf("getClient: %v", err)

		return err
	}

	client = c

	return nil
}

type request struct {
	SpreadsheetID string `json:"spreadsheet_id"`
	RangeValue    string `json:"range"`
}

type response struct {
	Request *request
	Values  [][]string
}

func readBody(in io.Reader) (*request, error) {
	var req request
	body, err := ioutil.ReadAll(in)

	if err != nil {
		log.Printf("iotuil.ReadAll: %v", err)
		return nil, err
	}
	if err := json.Unmarshal(body, &req); err != nil {
		log.Printf("json.Unmarshal: %v", err)
		return nil, err
	}
	return &req, nil
}
func Serve(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	if client == nil {
		if err := setUpClient(ctx); err != nil {
			log.Printf("setUpClient: %v", err)
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}
	}

	req, err := readBody(r.Body)
	if err != nil {
		log.Printf("readBody: %v", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	if req.SpreadsheetID == "" || req.RangeValue == "" {
		log.Printf("Not enough parameters: %+v", req)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	values, err := Load(ctx, client, req.SpreadsheetID, req.RangeValue)
	if err != nil {
		log.Printf("Load: %v", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	resp := &response{
		Request: req,
		Values:  values,
	}
	rb, err := json.Marshal(resp)
	if err != nil {
		log.Printf("json.Marshal: %v", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(rb)
}

func getAuthOption(ctx context.Context) (*option.ClientOption, error) {
	// If flags or env don't specify an auth source, try either gcloud or application default
	// credentials.
	creds, err := google.FindDefaultCredentials(ctx, scopes...)
	if creds != nil {
		log.Printf("Found google credentials for project: %s", creds.ProjectID)
		o := option.WithCredentials(creds)
		return &o, nil
	}

	log.Printf("Trying util.GcloudTokenSource")
	src, err := util.GcloudTokenSource(ctx)
	if err != nil {
		log.Printf("Trying goauth.DefaultTokenSource")
		src, err = goauth.DefaultTokenSource(ctx, scopes...)
	}
	if err != nil {
		return nil, err
	}
	o := option.WithTokenSource(src)
	return &o, nil
}
func getClient(ctx context.Context, secret string) (*http.Client, error) {
	ao, err := getAuthOption(ctx)
	if err != nil {
		log.Printf("getAuthOption: %v", err)
		return nil, err
	}
	sheetsCreds, err := getCredsFromSecretManager(ctx, ao, secret)
	if err != nil {
		log.Printf("getCredsFromSecretManager: %v", err)
		return nil, err
	}
	hc, err := authenticatedClient(ctx, sheetsCreds)
	if err != nil {
		log.Printf("authenticatedClient: %v", err)
		return nil, err
	}
	return hc, nil
}

func authenticatedClient(ctx context.Context, payload []byte) (*http.Client, error) {
	// First try and load this as a service account config, which allows us to see the service account email:
	if cfg, err := goauth.JWTConfigFromJSON(payload, scopes...); err == nil {
		log.Printf("using credential file for authentication; email=%s", cfg.Email)
		return cfg.Client(ctx), nil
	}
	// Try Oauth file
	//if cfg, err := goauth.ConfigFromJSON(payload, scopes...); err == nil {
	//	log.Printf("using oauth file for authentication; client_id=%s", cfg.ClientID)
	//	return croauth.GetClient(ctx, cfg), nil
	//}

	cred, err := goauth.CredentialsFromJSON(ctx, payload, scopes...)
	if err != nil {
		log.Printf("goauth.CredentialsFromJSON: %v", err)
		return nil, fmt.Errorf("invalid json file: %v", err)
	}
	log.Printf("using credential payload for project_id=%s", cred.ProjectID)
	return oauth2.NewClient(ctx, cred.TokenSource), nil
}

func getCredsFromSecretManager(ctx context.Context, o *option.ClientOption, secret string) ([]byte, error) {
	client, err := secretmanager.NewClient(ctx, *o)
	if err != nil {
		log.Printf("secretmanager.NewClient: %v", err)
		return nil, err
	}

	// Build the request.
	req := &secretmanagerpb.AccessSecretVersionRequest{
		Name: secret,
	}

	// Call the API.
	result, err := client.AccessSecretVersion(ctx, req)
	if err != nil {
		log.Printf("client.AccessSecretVersion: %v", err)
		return nil, err
	}
	return result.Payload.Data, nil
}

// Load - function load the data from the spreadsheet ID with the given range
// spreadsheetId = "10xtQZsNX0kjO0Jn6jUhBbzQuM5FRV6a_vZzY0NCjdyU"
// rangeValue = "'Form responses 1'!A2:K"
func Load(ctx context.Context, c *http.Client, spreadsheetId string, rangeValue string) ([][]string, error) {
	s, err := sheets.NewService(ctx, option.WithHTTPClient(c))
	if err != nil {
		log.Printf("sheets.NewService: %v", err)
		return nil, err
	}
	r, err := s.Spreadsheets.Values.Get(spreadsheetId, rangeValue).Context(ctx).Do()
	if err != nil {
		log.Printf("s.Spreadsheets.Values.Get: %v", err)
		return nil, err
	}
	b, err := json.Marshal(r.Values)
	if err != nil {
		log.Printf("json.Marshal: %v", err)
		return nil, err
	}
	err = ioutil.WriteFile("test.json", b, 0644)
	var response [][]string
	err = json.Unmarshal(b, &response)
	if err != nil {
		log.Printf("json.Unmarshal: %v", err)
		return nil, err
	}

	return response, nil
}
