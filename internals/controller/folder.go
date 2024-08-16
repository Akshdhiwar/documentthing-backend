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

func GetFolder(ctx *gin.Context) {
	id := ctx.Param("id")

	// parsing UUID for project id
	projectID, err := uuid.Parse(id)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"message": "Error parsing UUID" + err.Error(),
		})
		return
	}

	// getting details from DB
	var projectName, repoName string

	err = initializer.DB.QueryRow(context.Background(), `
	SELECT 
	    u.github_name,
	    p.name AS project_name
	FROM 
	    user_project_mapping upm
	JOIN 
	    users u ON upm.user_id = u.id
	JOIN 
	    projects p ON upm.project_id = p.id
	WHERE 
	    p.id = $1;
	`, projectID).Scan(&repoName, &projectName)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"message": "Error getting project details from DB : " + err.Error(),
		})
		return
	}

	content, err := getFolderJsonFromGithub(ctx, projectName, repoName)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, err)
		return
	}

	ctx.JSON(http.StatusOK, content)
}

func getFolderJsonFromGithub(ctx *gin.Context, repoName string, userName string) (string, error) {

	// Create a new HTTP request to GitHub API
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/simpledocs/folder/folder.json", userName, repoName)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create new HTTP request: %w", err)
	}

	// Set the Authorization header with the token from the request header
	req.Header.Set("Authorization", ctx.GetHeader("Authorization"))
	req.Header.Set("Content-Type", "application/json")

	// Make the HTTP request to GitHub API
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to make HTTP request: %w", err)
	}
	defer resp.Body.Close()

	// Handle response from GitHub API
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to get repository: %s", resp.Status)
	}

	// Decode the JSON response into a GitHubRepoResponse struct
	var githubResp githubContentResponse
	if err := json.NewDecoder(resp.Body).Decode(&githubResp); err != nil {
		return "", fmt.Errorf("failed to decode response body: %w", err)
	}

	// Return the content and SHA in a map
	return githubResp.Content, nil

}

type githubContentResponse struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Sha         string `json:"sha"`
	Size        int    `json:"size"`
	URL         string `json:"url"`
	HTMLURL     string `json:"html_url"`
	GitURL      string `json:"git_url"`
	DownloadURL string `json:"download_url"`
	Type        string `json:"type"`
	Content     string `json:"content"`
	Encoding    string `json:"encoding"`
	Links       Links  `json:"_links"`
}

type Links struct {
	Self string `json:"self"`
	Git  string `json:"git"`
	HTML string `json:"html"`
}

func UpdateFolder(ctx *gin.Context) {
	var body struct {
		FolderObject string `json:"folder_object"`
		ID           string `json:"id"`
		FileId       string `json:"file_id"`
	}

	err := ctx.ShouldBindJSON(&body)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, "Error while binding data")
		return
	}

	projectID, err := uuid.Parse(body.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, "Error while parsing project ID")
		return
	}

	var userName, repoName string

	err = initializer.DB.QueryRow(context.Background(), `
	SELECT 
	    u.github_name,
	    p.name AS project_name
	FROM 
	    user_project_mapping upm
	JOIN 
	    users u ON upm.user_id = u.id
	JOIN 
	    projects p ON upm.project_id = p.id
	WHERE 
	    p.id = $1;
	`, projectID).Scan(&userName, &repoName)

	if err != nil {
		ctx.JSON(http.StatusInternalServerError, "Error while query to DB :"+err.Error())
		return
	}

	// update folder structure
	err = updateFolderStructure(ctx, userName, repoName, body.FolderObject)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, err)
		return
	}

	// creating file
	err = createFile(ctx, userName, repoName, body.FileId)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, err)
		return
	}

	ctx.JSON(http.StatusCreated, "Folder Updated successfully")

}

func createFile(ctx *gin.Context, userName string, repoName string, fileId string) error {
	// Prepare the request body for GitHub API
	requestBody, err := json.Marshal(map[string]interface{}{
		"message": "created file " + fileId,
		"content": "", // Base64-encoded empty string for folder creation
	})
	if err != nil {
		return err
	}

	// The URL should point to the desired folder path, using an empty file name to create the folder
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/simpledocs/files/%s.json", userName, repoName, fileId)

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
		return errors.New("failed to create a file in repository")
	}

	return nil
}

func updateFolderStructure(ctx *gin.Context, userName string, repoName string, content string) error {

	// Get the latest SHA for the folder
	sha, err := getFolderSHA(ctx, repoName, userName)
	if err != nil {
		return err
	}

	// Prepare the request body for GitHub API
	requestBody, err := json.Marshal(map[string]interface{}{
		"message": "update folder",
		"content": content, // Base64-encoded empty string for folder creation
		"sha":     sha,
	})
	if err != nil {
		return err
	}

	// The URL should point to the desired folder path, using an empty file name to create the folder
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/simpledocs/folder/folder.json", userName, repoName)

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
	if resp.StatusCode != http.StatusOK {
		return errors.New("failed to update folder in repository")
	}

	return nil
}

func getFolderSHA(ctx *gin.Context, repoName string, userName string) (string, error) {

	// Create a new HTTP request to GitHub API
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/simpledocs/folder/folder.json", userName, repoName)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create new HTTP request: %w", err)
	}

	// Set the Authorization header with the token from the request header
	req.Header.Set("Authorization", ctx.GetHeader("Authorization"))
	req.Header.Set("Content-Type", "application/json")

	// Make the HTTP request to GitHub API
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to make HTTP request: %w", err)
	}
	defer resp.Body.Close()

	// Handle response from GitHub API
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to get repository: %s", resp.Status)
	}

	// Decode the JSON response into a GitHubRepoResponse struct
	var githubResp githubContentResponse
	if err := json.NewDecoder(resp.Body).Decode(&githubResp); err != nil {
		return "", fmt.Errorf("failed to decode response body: %w", err)
	}

	// Return the content and SHA in a map
	return githubResp.Sha, nil

}
