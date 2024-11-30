package utils

import (
	"bytes"
	"context"
	"database/sql"
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

func GetNewAccessTokenFromGithub(ctx *gin.Context, repoName, t string) {
	// Step 1: Get user ID from the header and fetch refresh token from the database
	id := ctx.GetHeader("X-User-Id")

	if t == "google" {
		err := initializer.DB.QueryRow(context.Background(), `
		SELECT
	u.id
  FROM
	public.projects p
	JOIN public.users u ON p.owner = u.id
  WHERE
	p.name = $1;
	`, repoName).Scan(&id)

		if err == sql.ErrNoRows {
			ctx.JSON(http.StatusNotFound, gin.H{"error": "Project not found"})
			return
		} else if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch user ID"})
			return
		}
	}

	var encRefreshToken string
	err := initializer.DB.QueryRow(context.Background(), `SELECT refresh_token FROM users WHERE id = $1`, id).Scan(&encRefreshToken)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "User not found"})
		return
	}

	// Step 2: Decrypt the refresh token
	key := DeriveKey(id + os.Getenv("ENC_SECRET"))
	refreshToken, err := Decrypt(encRefreshToken, key)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decrypt token"})
		return
	}

	// Step 3: Define the GitHub API URL and query parameters
	apiURL := "https://github.com/login/oauth/access_token"
	data := url.Values{}
	data.Set("client_id", os.Getenv("GITHUB_APP_CLIENT"))
	data.Set("client_secret", os.Getenv("GITHUB_APP_CLIENT_SECRET"))
	data.Set("refresh_token", refreshToken)
	data.Set("grant_type", "refresh_token")

	// Step 4: Create a POST request
	req, err := http.NewRequest("POST", apiURL, bytes.NewBufferString(data.Encode()))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create request"})
		return
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	// Step 5: Send the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get response from GitHub"})
		return
	}
	defer resp.Body.Close()

	// Step 6: Parse the response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read response body"})
		return
	}

	if resp.StatusCode != http.StatusOK {
		ctx.JSON(resp.StatusCode, gin.H{"error": string(body)})
		return
	}

	var tokenResponse struct {
		AccessToken           string `json:"access_token"`
		ExpiresIn             int    `json:"expires_in"`
		RefreshToken          string `json:"refresh_token"`
		RefreshTokenExpiresIn int    `json:"refresh_token_expires_in"`
		TokenType             string `json:"token_type"`
		Scope                 string `json:"scope"`
	}

	err = json.Unmarshal(body, &tokenResponse)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse response"})
		return
	}

	// Step 7: Encrypt and save the new refresh token in the database
	newEncRefreshToken, err := Encrypt([]byte(tokenResponse.RefreshToken), key)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to encrypt new refresh token"})
		return
	}

	// Step 7: Encrypt and save the new refresh token in the database
	newEncToken, err := Encrypt([]byte(tokenResponse.AccessToken), key)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to encrypt new refresh token"})
		return
	}

	_, err = initializer.DB.Exec(context.Background(), `UPDATE users SET refresh_token = $1 , token = $2 WHERE id = $3`, newEncRefreshToken, newEncToken, id)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save new refresh token"})
		return
	}

}
