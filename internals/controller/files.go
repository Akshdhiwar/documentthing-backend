package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/Akshdhiwar/simpledocs-backend/internals/initializer"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func GetFileContents(ctx *gin.Context) {
	projId := ctx.Query("proj")
	fileId := ctx.Query("file")

	if projId == "" || fileId == "" {
		ctx.JSON(http.StatusBadRequest, "Please provide required query")
		return
	}

	projectId, err := uuid.Parse(projId)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, "Error while parsing project id "+err.Error())
		return
	}

	fileID, err := uuid.Parse(fileId)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, "Error while parsing file id "+err.Error())
		return
	}

	// getting details from DB
	var projectName, userName string

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
	`, projectId).Scan(&userName, &projectName)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"message": "Error getting project details from DB : " + err.Error(),
		})
		return
	}

	content, err := getFileContentFromGithub(ctx, projectName, userName, fileID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, err.Error())
		return
	}

	ctx.JSON(http.StatusOK, content)

}

func getFileContentFromGithub(ctx *gin.Context, repoName string, repoAdmin string, fileId uuid.UUID) (string, error) {

	// Create a new HTTP request to GitHub API
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/simpledocs/files/%s.json", repoAdmin, repoName, fileId)
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

	return githubResp.Content, nil
}

func UpdateFileContents(ctx *gin.Context) {
	var body struct {
		ProjectID string `json:"project_id"`
		FileID    string `json:"file_id"`
		Content   string `json:"content"`
	}

	err := ctx.ShouldBindJSON(&body)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, "Error while binding body")
		return
	}

	projectId, err := uuid.Parse(body.ProjectID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, "Error while parsing project id "+err.Error())
		return
	}

	fileID, err := uuid.Parse(body.FileID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, "Error while parsing file id "+err.Error())
		return
	}

	// getting details from DB
	var projectName, userName string

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
	`, projectId).Scan(&userName, &projectName)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"message": "Error getting project details from DB : " + err.Error(),
		})
		return
	}

	err = saveContentIntoGithubFiles(ctx, fileID, projectName, userName, body.Content)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, err.Error())
		return
	}

	ctx.JSON(http.StatusOK, "Data saved successfully")

}

func saveContentIntoGithubFiles(ctx *gin.Context, fileID uuid.UUID, repoName string, repoAdmin string, content string) error {

	sha, err := getFileSha(ctx, repoName, repoAdmin, fileID)
	if err != nil {
		return fmt.Errorf("failed to get sha of file: %w", err)
	}

	// Prepare the request body for GitHub API
	requestBody, err := json.Marshal(map[string]interface{}{
		"message": "updated file content",
		"content": content, // Base64-encoded empty string for folder creation
		"sha":     sha,
	})
	if err != nil {
		return err
	}

	// Create a new HTTP request to GitHub API
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/simpledocs/files/%s.json", repoAdmin, repoName, fileID)
	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(requestBody))
	if err != nil {
		return fmt.Errorf("failed to create new HTTP request: %w", err)
	}

	// Set the Authorization header with the token from the request header
	req.Header.Set("Authorization", ctx.GetHeader("Authorization"))
	req.Header.Set("Content-Type", "application/json")

	// Make the HTTP request to GitHub API
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make HTTP request: %w", err)
	}
	defer resp.Body.Close()

	// Handle response from GitHub API
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to get repository: %s", resp.Status)
	}

	return nil
}

func getFileSha(ctx *gin.Context, repoName string, userName string, fileID uuid.UUID) (string, error) {

	// Create a new HTTP request to GitHub API
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/simpledocs/files/%s.json", userName, repoName, fileID)
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
