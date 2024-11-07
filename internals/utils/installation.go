package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// GenerateJWT generates a JWT token for a GitHub App
func GenerateJWT() (string, error) {
	// Get the private key from the environment variable
	privateKeyPEM := os.Getenv("GITHUB_APP_PRIVATE_KEY")
	if privateKeyPEM == "" {
		return "", fmt.Errorf("private key not found in environment variable")
	}

	// Parse the private key
	privateKey, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(privateKeyPEM))
	if err != nil {
		return "", fmt.Errorf("failed to parse private key: %v", err)
	}

	// Create the JWT claims, which include the GitHub App ID and issued times
	claims := jwt.MapClaims{
		"iat": time.Now().Unix(),                       // Issued at time
		"exp": time.Now().Add(time.Minute * 10).Unix(), // Expiration time (10 minutes)
		"iss": os.Getenv("GITHUB_APP_ID"),              // GitHub App ID
	}

	// Create the JWT
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)

	// Sign the JWT with the private key
	jwtToken, err := token.SignedString(privateKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign JWT: %v", err)
	}

	return jwtToken, nil
}

func GetInstallationAccessToken(id string, token string) (string, error) {

	url := fmt.Sprintf("https://api.github.com/app/installations/%s/access_tokens", id)

	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create new HTTP request: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to make HTTP request: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}
	var data map[string]interface{}
	if err = json.Unmarshal(body, &data); err != nil {
		return "", fmt.Errorf("failed to unmarshal JSON response: %w", err)
	}
	if resp.StatusCode != 201 {
		return "", fmt.Errorf("HTTP request failed with status code %d: %s", resp.StatusCode, string(body))
	}

	return data["token"].(string), nil

}
