package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/Eun/gdriver"
	"github.com/Eun/gdriver/oauthhelper"
)

func main() {
	// Setup OAuth
	helper := oauthhelper.Auth{
		ClientID:     "ClientID",
		ClientSecret: "ClientSecret",
		Authenticate: func(url string) (string, error) {
			fmt.Printf("Open to authorize Example to access your drive\n%s\n", url)

			var code string
			fmt.Printf("Code: ")
			if _, err := fmt.Scan(&code); err != nil {
				return "", fmt.Errorf("Unable to read authorization code %v", err)
			}
			return code, nil
		},
	}

	var err error
	// Try to load a client token from file
	helper.Token, err = oauthhelper.LoadTokenFromFile("token.json")
	if err != nil {
		// if the error is NotExist error continue
		// we will create a token
		if !os.IsNotExist(err) {
			log.Panic(err)
		}
	}

	// Create a new authorized HTTP client
	client, err := helper.NewHTTPClient(context.Background())
	if err != nil {
		log.Panic(err)
	}

	// store the token for future use
	if err = oauthhelper.StoreTokenToFile("token.json", helper.Token); err != nil {
		log.Panic(err)
	}

	// create a gdriver instance
	gdrive, err := gdriver.New(client)
	if err != nil {
		log.Panic(err)
	}

	err = gdrive.ListDirectory("/", func(info *gdriver.FileInfo) error {
		fmt.Printf("%s\t%d\t%s", info.Name(), info.Size(), info.ModifiedTime().String())
		return nil
	})
	if err != nil {
		log.Panic(err)
	}
}
