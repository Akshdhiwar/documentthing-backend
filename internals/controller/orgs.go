package controller

import (
	"context"
	"database/sql"
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
		Name        *string `json:"name"`
		ProjectName string  `json:"project_name"`
		GithubName  *string `json:"github_name"`
		Role        *string `json:"role"`
	}

	// Execute the query
	rows, err := initializer.DB.Query(context.Background(), `
		SELECT DISTINCT
			u.name,
			u.github_name,
			p.name AS project_name,
			opum.role AS role
		FROM 
			users u
		JOIN 
			org_user_mapping oum ON u.id = oum.user_id
		LEFT JOIN 
			org_project_user_mapping opum ON u.id = opum.user_id AND oum.org_id = opum.org_id
		LEFT JOIN 
			projects p ON opum.project_id = p.id
		WHERE 
			oum.org_id = $1
	`, orgID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve members"})
		return
	}
	defer rows.Close()

	// Iterate over the result set
	for rows.Next() {
		var member struct {
			Name        *string `json:"name"`
			ProjectName string  `json:"project_name"`
			GithubName  *string `json:"github_name"`
			Role        *string `json:"role"`
		}

		// Initialize fields with default values in case of NULLs
		var name, githubName, role sql.NullString
		var projectName sql.NullString

		// Scan each row into temporary variables
		if err := rows.Scan(&name, &githubName, &projectName, &role); err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan member"})
			return
		}

		// Assign values, setting to `nil` if they are NULL
		if name.Valid {
			member.Name = &name.String
		} else {
			member.Name = nil
		}
		if githubName.Valid {
			member.GithubName = &githubName.String
		} else {
			member.GithubName = nil
		}
		if projectName.Valid {
			member.ProjectName = projectName.String
		} else {
			member.ProjectName = ""
		}
		if role.Valid {
			member.Role = &role.String
		} else {
			member.Role = nil
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
