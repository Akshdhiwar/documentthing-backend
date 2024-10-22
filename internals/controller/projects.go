package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/Akshdhiwar/simpledocs-backend/internals/initializer"
	"github.com/Akshdhiwar/simpledocs-backend/internals/models"
	"github.com/Akshdhiwar/simpledocs-backend/internals/utils"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func CreateNewProject(ctx *gin.Context) {
	var body struct {
		Name  string `json:"name"`
		ID    string `json:"id"`
		Org   string `json:"org"`
		Owner string `json:"owner"`
		OrgID string `json:"org_id"`
	}

	// Bind JSON input to the body variable
	if err := ctx.ShouldBindJSON(&body); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	if body.Name == "" {
		ctx.JSON(http.StatusBadRequest, "Please provide name to create a project")
		return
	}

	// now creating repo in github
	// repo, err := createRepo(body.Name, ctx, body.Org)
	// if err != nil {
	// 	ctx.JSON(http.StatusInternalServerError, err.Error())
	// 	return
	// }

	// now saving the details in DB
	userId, err := uuid.Parse(body.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"message": "Error parsing UUID" + err.Error(),
		})
		return
	}

	orgId, err := uuid.Parse(body.OrgID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"message": "Error parsing UUID" + err.Error(),
		})
		return
	}

	tx, err := initializer.DB.Begin(context.Background())
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"message": "Unable to create transaction" + err.Error(),
		})
		return
	}

	// Ensure transaction is committed or rolled back
	defer func() {
		if err != nil {
			tx.Rollback(context.Background())
			// Delete GitHub repository if there was an error
			deleteRepo(body.Name, ctx)
		} else {
			tx.Commit(context.Background())
		}
	}()

	var projectID uuid.UUID

	err = tx.QueryRow(context.Background(), `INSERT INTO projects (name , owner , org , repo_owner) values ($1, $2 , $3 , $4) RETURNING id`, body.Name, body.ID, body.Org, body.Owner).Scan(&projectID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"message": "Error saving data to DB" + err.Error(),
		})
		return
	}

	_, err = tx.Exec(context.Background(), `INSERT INTO user_project_mapping (user_id , project_id) values ($1 , $2)`, userId, projectID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"message": "Error saving data to DB" + err.Error(),
		})
		return
	}

	var name string

	err = tx.QueryRow(context.Background(), "SELECT github_name FROM users WHERE id = $1", userId).Scan(&name)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"message": "Error getting user data from DB" + err.Error(),
		})
		return
	}

	// Create a entry in org-user-project-mapping table
	_, err = tx.Exec(context.Background(), `INSERT INTO org_project_user_mapping (org_id, user_id, project_id, role) VALUES ($1, $2, $3, $4)`, orgId, userId, projectID, "Owner")
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"message": "Error saving data to DB" + err.Error(),
		})
		return
	}

	folders := []string{"simpledocs", "simpledocs/files"}

	for _, folder := range folders {
		err := createRepoContents(body.Name, name, folder, ctx, body.Org)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	files := []string{"simpledocs/folder/folder.json"}

	for _, file := range files {
		err := createFilesContent(body.Name, name, file, ctx, body.Org)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	ctx.JSON(http.StatusCreated, gin.H{"message": "Repository created successfully"})
}

func createFilesContent(repoName string, name string, file string, ctx *gin.Context, org string) error {

	// Prepare the request body for GitHub API
	requestBody, err := json.Marshal(map[string]interface{}{
		"message": "initial commit",
		"content": "IltdIg==", // Base64-encoded empty string for folder creation
	})
	if err != nil {
		return err
	}

	// The URL should point to the desired folder path, using an empty file name to create the folder
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s", name, repoName, file)
	if org != "" {
		url = fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s", org, repoName, file)
	}

	// Create a new HTTP request to GitHub API
	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(requestBody))
	if err != nil {
		return err
	}

	token, err := utils.GetAccessTokenFromBackend(ctx)
	if err != nil {
		return err
	}

	// Set the Authorization header with the token from the request header
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("Content-Type", "application/json")

	// Make the HTTP request to GitHub API
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Handle response from GitHub API
	if resp.StatusCode != http.StatusCreated {
		return errors.New("failed to create folder in repository")
	}

	return nil
}

func createRepoContents(repoName string, name string, folder string, ctx *gin.Context, org string) error {

	// Prepare the request body for GitHub API
	requestBody, err := json.Marshal(map[string]interface{}{
		"message": "initial commit",
		"content": "", // Base64-encoded empty string for folder creation
	})
	if err != nil {
		return err
	}

	// The URL should point to the desired folder path, using an empty file name to create the folder
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s/.gitkeep", name, repoName, folder)
	if org != "" {
		url = fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s/.gitkeep", org, repoName, folder)
	}

	// Create a new HTTP request to GitHub API
	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(requestBody))
	if err != nil {
		return err
	}

	token, err := utils.GetAccessTokenFromBackend(ctx)
	if err != nil {
		return err
	}

	// Set the Authorization header with the token from the request header
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("Content-Type", "application/json")

	// Make the HTTP request to GitHub API
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Handle response from GitHub API
	if resp.StatusCode != http.StatusCreated {
		return errors.New("failed to create folder in repository")
	}

	return nil
}

func GetProjects(ctx *gin.Context) {
	// Retrieve name and id from the query parameters
	name := ctx.Query("name")
	idStr := ctx.Query("id")

	// Validate the parameters
	if name == "" || idStr == "" {
		ctx.JSON(http.StatusBadRequest, "Missing required query parameters: name or id")
		return
	}

	// Parse the UUID from the idStr
	id, err := uuid.Parse(idStr)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, "Error parsing the ID")
		return
	}

	repos, err := getAllRepos(ctx)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, err.Error())
		return
	}

	// Query the database for projects
	rows, err := initializer.DB.Query(ctx, `SELECT p.name , p.id , p.repo_owner , up.role
		FROM projects p 
		JOIN user_project_mapping up ON p.id = up.project_id 
		WHERE up.user_id = $1;`, id)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to query projects"})
		return
	}
	defer rows.Close()

	// Collect project names
	var projects []Project
	for rows.Next() {
		var project Project
		if err := rows.Scan(&project.Name, &project.Id, &project.RepoOwner, &project.Role); err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan project name"})
			return
		}
		projects = append(projects, project)
	}

	// Check for any errors from the row iteration
	if err := rows.Err(); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Error iterating over project names"})
		return
	}

	// check if name of project is present in repos and only send the required projects in api

	var requiredProjects []Project

	for i := 0; i < len(projects); i++ {
		for j := 0; j < len(repos); j++ {
			// Access the repository directly since repos[j] is of type models.Repository
			repo := repos[j]

			// Compare the name directly, no need for type assertion
			if repo.Name == projects[i].Name && repo.Owner.Login == projects[i].RepoOwner {
				var proj Project
				proj.Id = projects[i].Id
				proj.Name = projects[i].Name
				proj.Role = projects[i].Role
				proj.RepoOwner = projects[i].RepoOwner
				requiredProjects = append(requiredProjects, proj)
			}
		}
	}

	ctx.JSON(http.StatusOK, requiredProjects)

}

type Project struct {
	Name      string
	Id        uuid.UUID
	RepoOwner string
	Role      string
}

type GitHubRepoResponse struct {
	TotalCount        int           `json:"total_count"`
	IncompleteResults bool          `json:"incomplete_results"`
	Items             []interface{} `json:"items"`
}

func getAllRepos(ctx *gin.Context) ([]models.Repository, error) {
	// Create a new HTTP request to GitHub API
	url := "https://api.github.com/user/repos" // Note: Use "user/repos" for authenticated user's repos
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
	var githubResp []models.Repository
	if err := json.NewDecoder(resp.Body).Decode(&githubResp); err != nil {
		return nil, fmt.Errorf("failed to decode response body: %w", err)
	}

	return githubResp, nil
}

func createRepo(name string, ctx *gin.Context, org string) (models.Repository, error) {
	// Prepare the request body for GitHub API
	requestBody, err := json.Marshal(map[string]string{
		"name":    name,
		"private": "true",
	})
	if err != nil {
		return models.Repository{}, err
	}

	url := "https://api.github.com/user/repos"
	if org != "" {
		url = fmt.Sprintf("https://api.github.com/orgs/%s/repos", org)
	}

	// Create a new HTTP request to GitHub API
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(requestBody))
	if err != nil {
		return models.Repository{}, err
	}

	token, err := utils.GetAccessTokenFromBackend(ctx)
	if err != nil {
		return models.Repository{}, err
	}

	// Set the Authorization header with the token from the request header
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("Content-Type", "application/json")

	// Make the HTTP request to GitHub API
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return models.Repository{}, err
	}
	defer resp.Body.Close()

	// Handle response from GitHub API
	if resp.StatusCode != http.StatusCreated {
		return models.Repository{}, fmt.Errorf("failed to create repository")
	}

	// Decode the JSON response into a slice of Repository structs
	var githubResp models.Repository
	if err := json.NewDecoder(resp.Body).Decode(&githubResp); err != nil {
		return models.Repository{}, fmt.Errorf("failed to decode response body: %w", err)
	}

	return githubResp, nil
}

func deleteRepo(name string, ctx *gin.Context) error {
	// Prepare the request body for GitHub API
	requestBody, err := json.Marshal(map[string]string{
		"name": name,
	})
	if err != nil {
		return err
	}

	// Create a new HTTP request to GitHub API
	req, err := http.NewRequest("DELETE", "https://api.github.com/user/repos", bytes.NewBuffer(requestBody))
	if err != nil {
		return err
	}

	token, err := utils.GetAccessTokenFromBackend(ctx)
	if err != nil {
		return err
	}

	// Set the Authorization header with the token from the request header
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("Content-Type", "application/json")

	// Make the HTTP request to GitHub API
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Handle response from GitHub API
	if resp.StatusCode != http.StatusCreated {
		return errors.New("failed to delete repository")
	}

	return nil
}

func GetInstallation(ctx *gin.Context) {
	// Define the GitHub API URL
	url := "https://api.github.com/user/installations"

	// Create a new HTTP request to GitHub API
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Get the access token from your utility function
	token, err := utils.GetAccessTokenFromBackend(ctx)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get access token"})
		return
	}

	// Set the Authorization header with the Bearer token
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("Content-Type", "application/json")

	// Make the HTTP request to GitHub API
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to make HTTP request"})
		return
	}
	defer resp.Body.Close()

	// Handle the response from GitHub API
	if resp.StatusCode != http.StatusOK {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to get installation, status code: %d", resp.StatusCode)})
		return
	}

	// Decode the JSON response into an InstallationResponse struct
	var githubResp models.InstallationResponse
	if err := json.NewDecoder(resp.Body).Decode(&githubResp); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to decode response body"})
		return
	}

	// Create a custom response that filters out installation_id and login
	type InstallationInfo struct {
		InstallationID int    `json:"installation_id"`
		Login          string `json:"name"`
		Type           string `json:"type"`
	}

	var installationInfoList []InstallationInfo
	for _, installation := range githubResp.Installations {
		installationInfoList = append(installationInfoList, InstallationInfo{
			InstallationID: installation.ID,
			Login:          installation.Account.Login,
			Type:           installation.Account.Type,
		})
	}

	// Return the filtered data as JSON
	ctx.JSON(http.StatusOK, installationInfoList)
}

func GetAccessTokenForGithubAppInstallation(ctx *gin.Context) {
	id := ctx.Param("id")
	if id == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Installation ID is required"})
		return
	}

	appToken, err := utils.GenerateJWT()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate App JWT token"})
		return
	}

	installationAccessToken, err := utils.GetInstallationAccessToken(id, appToken)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get installation access token"})
		return
	}

	ctx.JSON(http.StatusOK, installationAccessToken)
}
