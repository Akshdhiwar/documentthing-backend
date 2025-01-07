package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/Akshdhiwar/simpledocs-backend/internals/initializer"
	"github.com/Akshdhiwar/simpledocs-backend/internals/utils"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

var EditingBranchesMappings = make(map[string]string)

func CreateBranchForEditing(ctx *gin.Context) {
	var body struct {
		ProjectID  string `json:"project_id"`
		BranchName string `json:"branch_name"`
	}

	userID := ctx.GetHeader("X-User-Id")

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

	// getting details from DB
	var projectName, userName, org string

	err = initializer.DB.QueryRow(context.Background(), `
		SELECT 
		u.github_name,
		p.name AS project_name,
		COALESCE(p.org, '') AS project_org
	FROM 
		user_project_mapping upm
	JOIN 
		users u ON upm.user_id = u.id
	JOIN 
		projects p ON upm.project_id = p.id
	WHERE 
		p.id = $1
		AND u.id = $2;
		`, projectId, userID).Scan(&userName, &projectName, &org)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"message": "Error getting project details from DB : " + err.Error(),
		})
		return
	}

	createBranch(ctx, projectName, userName, org, body.BranchName, body.ProjectID, userID)
}

func createBranch(ctx *gin.Context, projectName, userName, org, newBranch, projectID, userID string) {
	// Step 1: Get the SHA of the base branch
	latestCommistSha, err := getLatestShaFromGithub(ctx, projectName, userName, org, "main")
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, err.Error())
		return
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/git/refs", userName, projectName)

	if org != "" {
		url = fmt.Sprintf("https://api.github.com/repos/%s/%s/git/refs", org, projectName)
	}

	body, err := json.Marshal(map[string]interface{}{
		"ref": fmt.Sprintf("refs/heads/%s", newBranch),
		"sha": latestCommistSha,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, err.Error())
		return
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, err.Error())
		return
	}
	token, err := utils.GetAccessTokenFromBackend(ctx)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, err.Error())
		return
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		ctx.JSON(http.StatusInternalServerError, fmt.Sprintf("Error creating branch: %s", string(respBody)))
		return
	}

	EditingBranchesMappings[projectID+userID] = newBranch

	ctx.JSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("Branch %s created successfully", newBranch),
	})
}

func DeleteBranch(ctx *gin.Context) {

	projectID := ctx.Param("project_id")
	branchName := ctx.Param("branch_name")
	userID := ctx.GetHeader("X-User-Id")
	projectId, err := uuid.Parse(projectID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, "Error while parsing project id "+err.Error())
		return
	}

	// getting details from DB
	var projectName, userName, org string

	err = initializer.DB.QueryRow(context.Background(), `
		SELECT 
		u.github_name,
		p.name AS project_name,
		COALESCE(p.org, '') AS project_org
	FROM 
		user_project_mapping upm
	JOIN 
		users u ON upm.user_id = u.id
	JOIN 
		projects p ON upm.project_id = p.id
	WHERE 
		p.id = $1
		AND u.id = $2;
		`, projectId, userID).Scan(&userName, &projectName, &org)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"message": "Error getting project details from DB : " + err.Error(),
		})
		return
	}

	deleteBranch(ctx, projectName, userName, org, branchName, projectId.String(), userID)
}

func deleteBranch(ctx *gin.Context, projectName, userName, org, branchName, projectId, userID string) {
	// Construct the GitHub API URL for deleting the branch
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/git/refs/heads/%s", userName, projectName, branchName)
	if org != "" {
		url = fmt.Sprintf("https://api.github.com/repos/%s/%s/git/refs/heads/%s", org, projectName, branchName)
	}

	// Create the HTTP request
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create HTTP request", "details": err.Error()})
		return
	}

	// Retrieve the access token
	token, err := utils.GetAccessTokenFromBackend(ctx)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve access token", "details": err.Error()})
		return
	}
	req.Header.Set("Authorization", "token "+token)

	// Execute the HTTP request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to execute HTTP request", "details": err.Error()})
		return
	}
	defer resp.Body.Close()

	// Check if the response status is 204 No Content (success)
	if resp.StatusCode == http.StatusNoContent {
		ctx.JSON(http.StatusOK, gin.H{"success": "Branch deleted successfully"})
		delete(EditingBranchesMappings, projectId+userID)
		return
	}

	// Handle errors from GitHub API
	respBody, _ := io.ReadAll(resp.Body)
	ctx.JSON(resp.StatusCode, gin.H{
		"error":   "Failed to delete branch",
		"details": string(respBody),
	})
}

func CheckIfEditingBranchExists(ctx *gin.Context) {

	projectID := ctx.Param("id")
	userID := ctx.GetHeader("X-User-Id")

	projectId, err := uuid.Parse(projectID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, "Error while parsing project id "+err.Error())
		return
	}

	branchName, exists := EditingBranchesMappings[projectId.String()+userID]
	if !exists {
		ctx.JSON(http.StatusOK, gin.H{"branch_name": ""})
	} else {
		ctx.JSON(http.StatusOK, gin.H{"branch_name": branchName})
	}

}
