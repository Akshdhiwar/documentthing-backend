package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/Akshdhiwar/simpledocs-backend/internals/initializer"
	"github.com/Akshdhiwar/simpledocs-backend/internals/models"
	"github.com/Akshdhiwar/simpledocs-backend/internals/utils"
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
	token, err := utils.GetAccessTokenFromBackend(ctx)
	if err != nil {
		return nil, err
	}

	// Set the Authorization header with the token from the request header
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
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

func GetOrgMembersAdminOnly(ctx *gin.Context) {
	orgID := ctx.Param("id")

	// Define a slice of structs to hold multiple members
	var members []struct {
		ProjectName string `json:"project_name"`
		UserName    string `json:"user_name"`
		UserRole    string `json:"user_role"`
	}

	// Execute the query
	rows, err := initializer.DB.Query(context.Background(), `
		SELECT
			p.name AS project_name,
			u.name AS user_name,
			upm.role AS user_role
		FROM
			public.org_project_user_mapping opum
			LEFT JOIN public.projects p ON opum.project_id = p.id
			LEFT JOIN public.users u ON opum.user_id = u.id
			LEFT JOIN public.user_project_mapping upm ON opum.user_id = upm.user_id
			AND opum.project_id = upm.project_id
		WHERE
			opum.org_id = $1
	`, orgID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve members"})
		return
	}
	defer rows.Close()

	// Iterate over the result set
	for rows.Next() {
		var member struct {
			ProjectName string `json:"project_name"`
			UserName    string `json:"user_name"`
			UserRole    string `json:"user_role"`
		}

		// Scan each row into the member struct
		if err := rows.Scan(&member.ProjectName, &member.UserName, &member.UserRole); err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan member"})
			return
		}

		// Append each member to the members slice
		members = append(members, member)
	}

	// Check for any errors encountered during iteration
	if err := rows.Err(); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Error retrieving members"})
		return
	}

	ctx.JSON(http.StatusOK, members)
}
