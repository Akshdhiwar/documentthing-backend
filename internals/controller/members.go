package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/Akshdhiwar/simpledocs-backend/internals/initializer"
	"github.com/Akshdhiwar/simpledocs-backend/internals/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func GetOrgMembers(ctx *gin.Context) {
	id := ctx.Param("id")

	projectID, err := uuid.Parse(id)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, "Error while parsing project id "+err.Error())
		return
	}

	var org string

	err = initializer.DB.QueryRow(context.Background(), `SELECT org FROM projects WHERE id = $1`, projectID).Scan(&org)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, "Unable to Get org for specific project :"+err.Error())
		return
	}

	memebers, err, code := getAllMembersFormGithub(ctx, org)
	if err != nil {

		status := 500

		if code != 0 {
			status = code
		}

		ctx.JSON(status, err.Error())
		return
	}

	rows, err := initializer.DB.Query(context.Background(), `
    SELECT 
        upm.role,
        u.github_name
    FROM 
        user_project_mapping upm
    JOIN 
        users u ON upm.user_id = u.id
    WHERE 
        upm.project_id = $1;
		`, projectID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, fmt.Errorf("failed to execute query: %w", err))
		return
	}
	defer rows.Close()

	var users []struct {
		Role       string
		GithubName string
	}

	// Iterate over the rows
	for rows.Next() {
		var user struct {
			Role       string
			GithubName string
		}
		if err := rows.Scan(&user.Role, &user.GithubName); err != nil {
			ctx.JSON(http.StatusInternalServerError, fmt.Errorf("failed to scan row: %w", err))
			return
		}
		users = append(users, user)
	}

	// Check for errors after iterating through rows
	if err := rows.Err(); err != nil {
		ctx.JSON(http.StatusInternalServerError, fmt.Errorf("row iteration error: %w", err))
		return
	}

	var githubMember []struct {
		Name     string `json:"name"`
		Avatar   string `json:"avatar"`
		Role     string `json:"role"`
		IsActive string `json:"isActive"`
	}

	for _, member := range memebers {

		var temp struct {
			Name     string `json:"name"`
			Avatar   string `json:"avatar"`
			Role     string `json:"role"`
			IsActive string `json:"isActive"`
		}

		temp.Name = member.Name
		temp.Avatar = member.Avatar
		temp.IsActive = "-"
		temp.Role = "-"

		for _, u := range users {
			if member.Name == u.GithubName {
				temp.IsActive = "In project"
				temp.Role = u.Role
			}
		}

		githubMember = append(githubMember, temp)
	}

	ctx.JSON(http.StatusOK, githubMember)

}

func getAllMembersFormGithub(ctx *gin.Context, org string) ([]SubMember, error, int) {
	// Create a new HTTP request to GitHub API
	url := fmt.Sprintf("https://api.github.com/orgs/%s/members", org)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create new HTTP request: %w", err), 0
	}

	// Set the Authorization header with the token from the request header
	req.Header.Set("Authorization", ctx.GetHeader("Authorization"))
	req.Header.Set("Content-Type", "application/json")

	// Make the HTTP request to GitHub API
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make HTTP request: %w", err), 0
	}
	defer resp.Body.Close()

	// Handle response from GitHub API
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get members for this org: %s", resp.Status), resp.StatusCode
	}

	// Decode the JSON response into a GitHubRepoResponse struct
	var githubResp []models.Member
	if err := json.NewDecoder(resp.Body).Decode(&githubResp); err != nil {
		return nil, fmt.Errorf("failed to decode response body: %w", err), 0
	}

	var members []SubMember

	for _, member := range githubResp {
		var temp SubMember
		temp.Name = member.Login
		temp.Avatar = member.AvatarURL
		members = append(members, temp)
	}

	// Return the content and SHA in a map
	return members, nil, 0
}

type SubMember struct {
	Name   string
	Avatar string
}
