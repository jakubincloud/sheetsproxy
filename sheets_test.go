package sheetsproxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/go-cmp/cmp"
	"github.com/jakubincloud/sheetsproxy/sheetsproxy/util"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHelloHTTP(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)

	secretName = "projects/cr-lab-jzygmunt-2608185428/secrets/worker-svc/versions/latest"
	sheetId := "10xtQZsNX0kjO0Jn6jUhBbzQuM5FRV6a_vZzY0NCjdyU"
	rangeValue := "'Form responses 1'!A2:K"

	ctx := context.Background()
	err := setUpClient(ctx)
	if err != nil {
		t.Errorf("Cannot retrieve client data: %v", err)
	}
	response, err := Load(ctx, client, sheetId, rangeValue)
	if err != nil {
		t.Errorf("Cannot Load the spreadsheet: %v", err)
	}
	fmt.Printf("l: %d\n", len(response))
	fmt.Printf("%s\n", buf.String())
}

func TestServe(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)

	tests := []struct {
		body string
		want [][]string
	}{
		{body: `{"spreadsheet_id": "10xtQZsNX0kjO0Jn6jUhBbzQuM5FRV6a_vZzY0NCjdyU", "range": "'Form responses 1'!A1:K1"}`, want: [][]string{
			{"Timestamp", "Email address", "Platform", "Purpose", "DeleteOn", "AccountId", "Status", "Notes", "CreationDate", "CreationPlatform", "PreviousDeleteOn"},
		}},
	}

	for _, test := range tests {
		req := httptest.NewRequest("GET", "/", strings.NewReader(test.body))
		req.Header.Add("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		Serve(rr, req)

		got, err := readResponse(rr.Body)
		if err != nil {
			t.Errorf("Error reading the response")
			continue
		}
		if !cmp.Equal(got.Values, test.want) {
			t.Errorf("HelloHTTP(%q) = %q, want %q", test.body, got.Values, test.want)
		}

	}
	fmt.Printf("%s\n", buf.String())
}

func readResponse(in io.Reader) (*response, error) {
	var req response
	body, err := ioutil.ReadAll(in)
	log.Printf("%s", body)
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

func TestAuthenticatedCloudF(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)

	secretName = "projects/cr-lab-jzygmunt-2608185428/secrets/worker-svc/versions/latest"
	sheetsProxyUrl := "https://us-central1-cr-lab-jzygmunt-2608185428.cloudfunctions.net/sheets-proxy"
	serviceAccount := "worker@cr-lab-jzygmunt-2608185428.iam.gserviceaccount.com"

	ctx := context.Background()
	err := setUpClient(ctx)
	if err != nil {
		t.Errorf("Cannot retrieve client data: %v", err)
		return
	}
	// the ID token will be valid for 1 hr
	idToken, err := util.GetIDTokenForEndpoint(ctx, client, serviceAccount, sheetsProxyUrl)
	if err != nil {
		t.Errorf("Cannot get ID token: %v", err)
		return
	}

	body, err := json.Marshal(map[string]string{"spreadsheet_id": "10xtQZsNX0kjO0Jn6jUhBbzQuM5FRV6a_vZzY0NCjdyU", "range": "'Form responses 1'!A1:K1"})
	if err != nil {
		t.Errorf("cannot marshal body")
	}

	req, err := http.NewRequest("POST", sheetsProxyUrl, bytes.NewBuffer(body))
	req.Header.Add("Authorization", "Bearer "+idToken)
	req.Header.Add("Content-Type", "application/json")
	// Create a new Client
	netClient := &http.Client{
		Timeout: time.Second * 10,
	}
	r, err := netClient.Do(req)
	if err != nil {
		log.Printf("client.Do: %v", err)
		t.Errorf("client.Do: %v", err)
	}
	response, err := readResponse(r.Body)

	if err != nil {
		t.Errorf("cannot parse response: %v", err)
	}
	fmt.Printf("response: %v", response)
	fmt.Printf("%s\n", buf.String())
}
