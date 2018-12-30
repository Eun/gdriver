package oauthhelper

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"golang.org/x/oauth2"
)

type AuthenticateFunc func(url string) (code string, err error)

type Auth struct {
	// Token holds the token that should be used for authentication (optional)
	// if the token is nil the callback func Authenticate will be called and after Authorization this token will be set
	// Store (and restore prior use) this token to avoid further authorization calls
	Token *oauth2.Token
	// ClientID  from https://console.developers.google.com/project/<your-project-id>/apiui/credential
	ClientID string
	// ClientSecret  from https://console.developers.google.com/project/<your-project-id>/apiui/credential
	ClientSecret string
	Authenticate AuthenticateFunc
}

func (auth *Auth) NewHTTPClient(ctx context.Context) (*http.Client, error) {
	config := &oauth2.Config{
		Scopes:      []string{"https://www.googleapis.com/auth/drive"},
		RedirectURL: "urn:ietf:wg:oauth:2.0:oob",
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://accounts.google.com/o/oauth2/auth",
			TokenURL: "https://accounts.google.com/o/oauth2/token",
		},
		ClientID:     auth.ClientID,
		ClientSecret: auth.ClientSecret,
	}

	if auth.Token == nil {
		var err error
		auth.Token, err = auth.getTokenFromWeb(config)
		if err != nil {
			return nil, err
		}
	}

	return config.Client(ctx, auth.Token), nil
}

func (auth *Auth) getTokenFromWeb(config *oauth2.Config) (*oauth2.Token, error) {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	code, err := auth.Authenticate(authURL)
	if err != nil {
		return nil, fmt.Errorf("Authenticate error: %v", err)
	}
	tok, err := config.Exchange(oauth2.NoContext, code)
	if err != nil {
		return nil, fmt.Errorf("Unable to retrieve token from web %v", err)
	}
	return tok, nil
}

func LoadTokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	var token oauth2.Token
	if err = json.NewDecoder(f).Decode(&token); err != nil {
		return nil, fmt.Errorf("Unable to decode token: %v", err)
	}
	f.Close()
	return &token, nil
}

func StoreTokenToFile(file string, token *oauth2.Token) error {
	f, err := os.Create(file)
	if err != nil {
		return err
	}
	if err = json.NewEncoder(f).Encode(token); err != nil {
		return fmt.Errorf("Unable to encode token: %v", err)
	}
	f.Close()
	return nil
}
