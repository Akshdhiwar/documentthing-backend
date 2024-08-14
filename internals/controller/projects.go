package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/Akshdhiwar/simpledocs-backend/internals/initializer"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func CreateNewProject(ctx *gin.Context) {
	var body struct {
		Name string `json:"name"`
		ID   string `json:"id"`
	}

	// Bind JSON input to the body variable
	if err := ctx.ShouldBindJSON(&body); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}

	// now creating repo in github
	err := createRepo(body.Name, ctx)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, err.Error())
	}

	// now saving the details in DB
	userId, err := uuid.Parse(body.ID)
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

	err = tx.QueryRow(context.Background(), `INSERT INTO projects (name , owner) values ($1, $2) RETURNING id`, body.Name, body.ID).Scan(&projectID)
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

	folders := []string{"markdown", "html", "plaintext", "simpledocs", "simpledocs/files"}

	for _, folder := range folders {
		err := createRepoContents(body.Name, name, folder, ctx)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	files := []string{"simpledocs/folder/folder.json"}

	for _, file := range files {
		err := createFilesContent(body.Name, name, file, ctx)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	ctx.JSON(http.StatusCreated, gin.H{"message": "Repository created successfully"})
}

func createFilesContent(repoName string, name string, file string, ctx *gin.Context) error {

	// Prepare the request body for GitHub API
	requestBody, err := json.Marshal(map[string]interface{}{
		"message": "initial commit",
		"content": "W10=", // Base64-encoded empty string for folder creation
	})
	if err != nil {
		return err
	}

	// The URL should point to the desired folder path, using an empty file name to create the folder
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s", name, repoName, file)

	// Create a new HTTP request to GitHub API
	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(requestBody))
	if err != nil {
		return err
	}

	// Set the Authorization header with the token from the request header
	req.Header.Set("Authorization", ctx.GetHeader("Authorization"))
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

func createRepoContents(repoName string, name string, folder string, ctx *gin.Context) error {

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

	// Create a new HTTP request to GitHub API
	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(requestBody))
	if err != nil {
		return err
	}

	// Set the Authorization header with the token from the request header
	req.Header.Set("Authorization", ctx.GetHeader("Authorization"))
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

	repos, err := getAllRepos(name, ctx)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, err.Error())
		return
	}

	// Query the database for projects
	rows, err := initializer.DB.Query(ctx, `SELECT p.name , p.id
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
		if err := rows.Scan(&project.Name, &project.Id); err != nil {
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
			// Assert that repos[j] is a map[string]interface{}
			repoMap, ok := repos[j].(map[string]interface{})
			if !ok {
				// Handle the case where the type assertion fails
				fmt.Println("Type assertion failed for repo:", repos[j])
				continue
			}

			// Assert that the "name" key exists and is of type string
			name, ok := repoMap["name"].(string)
			if ok && name == projects[i].Name {

				var proj Project

				proj.Id = projects[i].Id
				proj.Name = projects[i].Name

				requiredProjects = append(requiredProjects, proj)
			}
		}
	}

	ctx.JSON(http.StatusOK, requiredProjects)

}

type Project struct {
	Name string
	Id   uuid.UUID
}

type GitHubRepoResponse struct {
	TotalCount        int           `json:"total_count"`
	IncompleteResults bool          `json:"incomplete_results"`
	Items             []interface{} `json:"items"`
}

func getAllRepos(name string, ctx *gin.Context) ([]interface{}, error) {
	// Create a new HTTP request to GitHub API
	url := fmt.Sprintf("https://api.github.com/search/repositories?q=user:%s", name)
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

	// Decode the JSON response into a GitHubRepoResponse struct
	var githubResp GitHubRepoResponse
	if err := json.NewDecoder(resp.Body).Decode(&githubResp); err != nil {
		return nil, fmt.Errorf("failed to decode response body: %w", err)
	}

	return githubResp.Items, nil
}

func createRepo(name string, ctx *gin.Context) error {
	// Prepare the request body for GitHub API
	requestBody, err := json.Marshal(map[string]string{
		"name":    name,
		"private": "true",
	})
	if err != nil {
		return err
	}

	// Create a new HTTP request to GitHub API
	req, err := http.NewRequest("POST", "https://api.github.com/user/repos", bytes.NewBuffer(requestBody))
	if err != nil {
		return err
	}

	// Set the Authorization header with the token from the request header
	req.Header.Set("Authorization", ctx.GetHeader("Authorization"))
	req.Header.Set("Content-Type", "application/json")

	// Make the HTTP request to GitHub API
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Handle response from GitHub API
	if resp.StatusCode != http.StatusCreated {
		return errors.New("failed to create repository")
	}

	return nil
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

	// Set the Authorization header with the token from the request header
	req.Header.Set("Authorization", ctx.GetHeader("Authorization"))
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
