package utils

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"

	"github.com/Akshdhiwar/simpledocs-backend/internals/initializer"
	"github.com/gin-gonic/gin"
)

var PaypalAccessToken string

func GetAccessTokenFromBackend(ctx *gin.Context) (string, error) {

	id := ctx.GetHeader("X-User-Id")

	var encryptedToken, name string
	var githubID int

	err := initializer.DB.QueryRow(context.Background(), `SELECT token , github_name , github_id from users WHERE id = $1`, id).Scan(&encryptedToken, &name, &githubID)
	if err != nil {
		return "", err
	}

	key := DeriveKey(id + os.Getenv("ENC_SECRET"))

	token, err := Decrypt(encryptedToken, key)
	if err != nil {
		return "", err
	}

	return token, nil
}

func GetPaypalAccessToken() {
	clientID := os.Getenv("PAYPAL_CLIENT_ID")
	clientSecret := os.Getenv("PAYPAL_CLIENT_SECRET")

	apiURL := "https://api-m.sandbox.paypal.com/v1/oauth2/token"

	// Prepare data
	data := url.Values{}
	data.Set("grant_type", "client_credentials")

	// Encode CLIENT_ID:CLIENT_SECRET in Base64
	auth := base64.StdEncoding.EncodeToString([]byte(clientID + ":" + clientSecret))

	// Create the request
	req, err := http.NewRequest("POST", apiURL, bytes.NewBufferString(data.Encode()))
	if err != nil {
		fmt.Println("Error creating request:", err)
		return
	}

	// Set headers
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Basic "+auth)

	// Send the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error sending request:", err)
		return
	}
	defer resp.Body.Close()

	// Parse the JSON response to extract the access token
	var result map[string]interface{}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading response:", err)
		return
	}

	err = json.Unmarshal(body, &result)
	if err != nil {
		fmt.Println("Error parsing JSON response:", err)
		return
	}

	// Extract and print the access token
	if accessToken, ok := result["access_token"].(string); ok {
		PaypalAccessToken = accessToken
	} else {
		fmt.Println("Access token not found in response.")
	}
}
