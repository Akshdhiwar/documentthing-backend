package controller

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/Akshdhiwar/simpledocs-backend/internals/models"
	"github.com/gin-gonic/gin"
)

func GetOrganization(ctx *gin.Context) {
	orgs, err := getOrgs(ctx)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, err.Error())
		return
	}

	ctx.JSON(http.StatusOK, orgs)
}

func getOrgs(ctx *gin.Context) ([]models.Organization, error) {
	// Create a new HTTP request to GitHub API
	url := "https://api.github.com/user/orgs" // Note: Use "user/orgs" for authenticated user's orgs
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create new HTTP request: %w", err)
	}

	// Set the Authorization header with the token from the request header
	req.Header.Set("Authorization", ctx.GetHeader("Authorization"))
	req.Header.Set("Content-Type", "application/json")

	// Make the HTTP request to GitHub API
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make HTTP request: %w", err)
	}
	defer resp.Body.Close()

	// Handle response from GitHub API
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get repository: %s", resp.Status)
	}

	// Decode the JSON response into a slice of Repository structs
	var githubResp []models.Organization
	if err := json.NewDecoder(resp.Body).Decode(&githubResp); err != nil {
		return nil, fmt.Errorf("failed to decode response body: %w", err)
	}

	return githubResp, nil
}
