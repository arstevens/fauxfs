package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

const FileIDLength = 8

// Interface describing any networked drive
type NetDrive interface {
	// Download the file with fileID to out
	Download(fileID string, out io.Writer) error

	// Upload the file from in and return its fileID
	Upload(in io.Reader) (string, error)

	// Return used space and total space in bytes
	GetSpace() (int64, int64, error)
}

// Google Drive implementation of NetDrive
type GoogleDrive struct {
	config  *oauth2.Config
	service *drive.Service
}

func (g *GoogleDrive) GetSpace() (int64, int64, error) {
	about, err := g.service.About.Get().Do()
	if err != nil {
		return -1, -1, fmt.Errorf("Failed to get google drive info: %v", err)
	}
	return about.StorageQuota.Usage, about.StorageQuota.Limit, nil
}

func (g *GoogleDrive) Download(fileID string, out io.Writer) error {
	exportCall := g.service.Files.Export(fileID, "application/octet-stream")
	response, err := exportCall.Download()
	if err != nil {
		return fmt.Errorf("Failed to download %s: %v", fileID, err)
	}

	if _, err = io.Copy(out, response.Body); err != nil {
		return fmt.Errorf("Failed to copy response body for %s: %v", fileID, err)
	}
	return nil
}

func (g *GoogleDrive) Upload(in io.Reader) (string, error) {
	createCall := g.service.Files.Create(&drive.File{
		Name:     generateRandomID(FileIDLength),
		MimeType: "application/octet-stream",
	}).Media(in)

	file, err := createCall.Do()
	if err != nil {
		return "", fmt.Errorf("Failed to upload file: %v", err)
	}
	return file.Id, nil
}

/*
Create new GoogleDrive with "credentials.json" and "token.json" files located in credsDir.
If token.json does not exist, user will have to undergo authentication
*/
func NewGoogleDrive(credsDir string) (*GoogleDrive, error) {
	credsFile := filepath.Join(credsDir, "credentials.json")
	b, err := os.ReadFile(credsFile)
	if err != nil {
		return nil, fmt.Errorf("Failed to read credentials file: %v", err)
	}

	config, err := google.ConfigFromJSON(b, drive.DriveFileScope)
	if err != nil {
		return nil, fmt.Errorf("Failed to create google credentials: %v", err)
	}

	tokFile := filepath.Join(credsDir, "token.json")
	client, err := getClient(tokFile, config)
	if err != nil {
		return nil, fmt.Errorf("Failed to get client: %v", err)
	}

	service, err := drive.NewService(context.Background(), option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("Failed to create drive client: %v", err)
	}

	return &GoogleDrive{
		config:  config,
		service: service,
	}, nil
}

func generateRandomID(n int) string {
	letters := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890")

	buf := make([]rune, n)
	for i := range buf {
		buf[i] = letters[rand.Intn(len(letters))]
	}
	return string(buf)
}

func getClient(tokFile string, config *oauth2.Config) (*http.Client, error) {
	tok, err := getTokenFromFile(tokFile)
	if err != nil {
		tok, err = getTokenFromWeb(config)
		if err != nil {
			return nil, err
		}
		if err = saveToken(tokFile, tok); err != nil {
			return nil, err
		}
	}
	return config.Client(context.Background(), tok), nil
}

func saveToken(path string, token *oauth2.Token) error {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("Failed to save token to %s: %v", path, err)
	}
	defer f.Close()

	if err = json.NewEncoder(f).Encode(token); err != nil {
		return fmt.Errorf("Failed to encode token to path %s: %v", path, err)
	}
	return nil
}

func getTokenFromFile(path string) (*oauth2.Token, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("Failed to open token file %s: %v", path, err)
	}
	defer f.Close()

	tok := &oauth2.Token{}
	if err = json.NewDecoder(f).Decode(tok); err != nil {
		return nil, fmt.Errorf("Failed to decode token from file %s: %v", path, err)
	}
	return tok, nil
}

func getTokenFromWeb(config *oauth2.Config) (*oauth2.Token, error) {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the authorization code: \n%v\n", authURL)

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		return nil, fmt.Errorf("Failed to read console input: %v", err)
	}

	tok, err := config.Exchange(context.TODO(), authCode)
	if err != nil {
		return nil, fmt.Errorf("Failed to retrieve token from web: %v", err)
	}
	return tok, nil
}
