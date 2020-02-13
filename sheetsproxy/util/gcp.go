package util

import (
	"context"
	"google.golang.org/api/iamcredentials/v1"
	"google.golang.org/api/option"
	"log"
	"net/http"
)

func GetIDTokenForEndpoint(ctx context.Context, client *http.Client, serviceAccount string, endpointUrl string) (string, error) {
	s, err := iamcredentials.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		log.Printf("iamcredentials.NewService: %v", err)
		return "", err
	}
	sc := iamcredentials.NewProjectsServiceAccountsService(s)

	gr := &iamcredentials.GenerateIdTokenRequest{
		Audience:     endpointUrl,
		Delegates:    nil,
		IncludeEmail: true,
	}
	name := "projects/-/serviceAccounts/" + serviceAccount
	r, err := sc.GenerateIdToken(name, gr).Do()
	if err != nil {
		log.Printf("GenerateIdToken")
	}
	return r.Token, nil
}
