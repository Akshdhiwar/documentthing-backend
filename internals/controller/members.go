package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/Akshdhiwar/simpledocs-backend/internals/initializer"
	"github.com/Akshdhiwar/simpledocs-backend/internals/models"
	"github.com/Akshdhiwar/simpledocs-backend/internals/utils"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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

	memebers, code, err := getAllMembersFormGithub(ctx, org)
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

func getAllMembersFormGithub(ctx *gin.Context, org string) ([]SubMember, int, error) {
	// Create a new HTTP request to GitHub API
	url := fmt.Sprintf("https://api.github.com/orgs/%s/members", org)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create new HTTP request: %w", err)
	}

	token, err := utils.GetAccessTokenFromBackend(ctx)
	if err != nil {
		return nil, 0, err
	}

	// Set the Authorization header with the token from the request header
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("Content-Type", "application/json")

	// Make the HTTP request to GitHub API
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to make HTTP request: %w", err)
	}
	defer resp.Body.Close()

	// Handle response from GitHub API
	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, fmt.Errorf("failed to get members for this org: %s", resp.Status)
	}

	// Decode the JSON response into a GitHubRepoResponse struct
	var githubResp []models.Account
	if err := json.NewDecoder(resp.Body).Decode(&githubResp); err != nil {
		return nil, 0, fmt.Errorf("failed to decode response body: %w", err)
	}

	var members []SubMember

	for _, member := range githubResp {
		var temp SubMember
		temp.Name = member.Login
		temp.Avatar = member.AvatarURL
		members = append(members, temp)
	}

	getOrganizationMembersEmails(ctx, org)

	// Return the content and SHA in a map
	return members, 0, nil
}

type SubMember struct {
	Name   string
	Avatar string
}

func GetUserDetails(ctx *gin.Context) {

	userName := ctx.Param("name")
	id := ctx.Param("proj")

	projectID, err := uuid.Parse(id)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, "Error while parsing the uuid")
		return
	}

	if userName == "" {
		ctx.JSON(http.StatusBadRequest, "No params sent in url")
		return
	}

	user, err := getUserDetailsFormGithub(ctx, userName)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, err.Error())
		return
	}

	err = initializer.DB.QueryRow(context.Background(), `
    SELECT 
        upm.role
    FROM 
        user_project_mapping upm
    JOIN 
        users u ON upm.user_id = u.id
    WHERE 
        upm.project_id = $1
        AND u.github_name = $2;
	`, projectID, userName).Scan(&user.Role)

	if err != nil {
		if err == pgx.ErrNoRows {
			user.Role = "" // Assign empty string if no row is found
			err = nil      // Reset the error since it's handled
		} else {
			ctx.JSON(http.StatusInternalServerError, "Error while retriving data from Database")
			return
		}
	}

	var inviteExists bool

	err = initializer.DB.QueryRow(context.Background(), `
    SELECT
        CASE
            WHEN EXISTS (
                SELECT 1
                FROM invite
                WHERE 
                    user_name = $1
                    AND project_id = $2
                    AND deleted_at IS NULL
                    AND is_accepted IS FALSE
                    AND is_revoked IS FALSE
            ) 
            THEN TRUE
            ELSE FALSE
        END AS invite_exists;
	`, userName, projectID).Scan(&inviteExists)

	if err != nil {
		ctx.JSON(http.StatusInternalServerError, "Error while retriving invite data from Database")
		return
	}

	var tempUser struct {
		Name       *string `json:"name"`
		Avatar     string  `json:"avatar"`
		GithubName string  `json:"githubName"`
		Email      *string `json:"email"`
		Twitter    *string `json:"twitter"`
		Role       string  `json:"role"`
		Company    string  `json:"company"`
		IsActive   *string `json:"isActive"`
		ID         int     `json:"id"`
	}

	tempUser.Name = user.Name
	tempUser.Avatar = user.AvatarURL
	tempUser.GithubName = user.Login
	tempUser.Email = user.Email
	tempUser.Twitter = user.TwitterUsername
	tempUser.Role = user.Role
	// Set IsActive field based on the Role
	if user.Role != "" {
		activeStatus := "In Project"
		tempUser.IsActive = &activeStatus
	} else {
		tempUser.IsActive = nil
	}

	if user.Role == "" && inviteExists {
		activeStatus := "Invite has been sent. You can make another invite after 48hr from invitation time."
		tempUser.IsActive = &activeStatus
	}
	tempUser.ID = user.ID

	if err != nil {
		if err == pgx.ErrNoRows {
			ctx.JSON(http.StatusOK, tempUser)
			return
		} else {
			ctx.JSON(http.StatusInternalServerError, fmt.Sprintf("Error executing query: %v", err))
			return
		}
	}

	ctx.JSON(http.StatusOK, tempUser)

}

func getUserDetailsFormGithub(ctx *gin.Context, name string) (models.ExtendedGitHubUser, error) {
	var githubResp models.GitHubUser

	// Create a new HTTP request to GitHub API
	url := fmt.Sprintf("https://api.github.com/users/%s", name)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return models.ExtendedGitHubUser{}, fmt.Errorf("failed to create new HTTP request: %w", err)
	}

	token, err := utils.GetAccessTokenFromBackend(ctx)
	if err != nil {
		return models.ExtendedGitHubUser{}, err
	}

	// Set the Authorization header with the token from the request header
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("Content-Type", "application/json")

	// Make the HTTP request to GitHub API
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return models.ExtendedGitHubUser{}, fmt.Errorf("failed to make HTTP request: %w", err)
	}
	defer resp.Body.Close()

	// Handle response from GitHub API
	if resp.StatusCode != http.StatusOK {
		return models.ExtendedGitHubUser{}, fmt.Errorf("failed to get members for this org: %s", resp.Status)
	}

	// Decode the JSON response into a GitHubRepoResponse struct

	if err := json.NewDecoder(resp.Body).Decode(&githubResp); err != nil {
		return models.ExtendedGitHubUser{}, fmt.Errorf("failed to decode response body: %w", err)
	}

	return models.ExtendedGitHubUser{
		GitHubUser: githubResp,
		Role:       "",
	}, nil
}

func getOrganizationMembersEmails(ctx *gin.Context, orgName string) {
	query := fmt.Sprintf(`
		query {
			organization(login: "%s") {
				membersWithRole(first: 100) {
					edges {
						node {
							login
							email
						}
					}
				}
			}
		}
	`, orgName)

	reqBody := map[string]string{"query": query}
	reqBodyBytes, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", "https://api.github.com/graphql", bytes.NewBuffer(reqBodyBytes))
	token, err := utils.GetAccessTokenFromBackend(ctx)
	if err != nil {
		return
	}

	// Set the Authorization header with the token from the request header
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	defer resp.Body.Close()

	var graphqlResp struct {
		Data struct {
			Organization struct {
				MembersWithRole struct {
					Edges []struct {
						Node struct {
							Login string `json:"login"`
							Email string `json:"email"`
						} `json:"node"`
					} `json:"edges"`
				} `json:"membersWithRole"`
			} `json:"organization"`
		}
	}

	if err := json.NewDecoder(resp.Body).Decode(&graphqlResp); err != nil {
		fmt.Println("Error decoding response:", err)
		return
	}

	for _, edge := range graphqlResp.Data.Organization.MembersWithRole.Edges {
		fmt.Printf("Login: %s, Email: %s\n", edge.Node.Login, edge.Node.Email)
	}
}
