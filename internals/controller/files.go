package controller

import (
	"bytes"
	"context"
	"encoding/base64"
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

func GetFileContents(ctx *gin.Context) {
	projId := ctx.Query("proj")
	fileId := ctx.Query("file")
	t := ctx.Query("t")
	userID := ctx.GetHeader("X-User-Id")

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
	var projectName, userName, org string

	if t == "google" {
		err = initializer.DB.QueryRow(context.Background(), `SELECT
  			repo_owner,
  				NAME,
  				COALESCE(org, '') AS org
				FROM
  				projects
				WHERE
  			id = $1;`, projectId).Scan(&userName, &projectName, &org)

		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{
				"message": "Error getting project details from DB : " + err.Error(),
			})
			return
		}

	} else {
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
	}

	content, err := getFileContentFromGithub(ctx, projectName, userName, fileID, org, t)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, err.Error())
		return
	}

	ctx.JSON(http.StatusOK, content)

}

func getFileContentFromGithub(ctx *gin.Context, repoName string, repoAdmin string, fileId uuid.UUID, org string, t string) (string, error) {

	// Create a new HTTP request to GitHub API
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/Documentthing/files/%s.json", repoAdmin, repoName, fileId)
	if org != "" {
		url = fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/Documentthing/files/%s.json", org, repoName, fileId)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create new HTTP request: %w", err)
	}

	token, err := GetAccessTokenFromBackendTypeGoogle(ctx, t, repoName)
	if err != nil {
		return "", err
	}

	// Set the Authorization header with the token from the request header
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
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
	    p.id = $1;
	`, projectId).Scan(&userName, &projectName, &org)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"message": "Error getting project details from DB : " + err.Error(),
		})
		return
	}

	err = saveContentIntoGithubFiles(ctx, fileID, projectName, userName, body.Content, org)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, err.Error())
		return
	}

	ctx.JSON(http.StatusOK, "Data saved successfully")

}

func saveContentIntoGithubFiles(ctx *gin.Context, fileID uuid.UUID, repoName string, repoAdmin string, content string, org string) error {

	sha, err := getFileSha(ctx, repoName, repoAdmin, fileID, org)
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
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/Documentthing/files/%s.json", repoAdmin, repoName, fileID)

	if org != "" {
		url = fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/Documentthing/files/%s.json", org, repoName, fileID)
	}

	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(requestBody))
	if err != nil {
		return fmt.Errorf("failed to create new HTTP request: %w", err)
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
		return fmt.Errorf("failed to make HTTP request: %w", err)
	}
	defer resp.Body.Close()

	// Handle response from GitHub API
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to get repository: %s", resp.Status)
	}

	return nil
}

func getFileSha(ctx *gin.Context, repoName string, userName string, fileID uuid.UUID, org string) (string, error) {

	// Create a new HTTP request to GitHub API
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/Documentthing/files/%s.json", userName, repoName, fileID)

	if org != "" {
		url = fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/Documentthing/files/%s.json", org, repoName, fileID)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create new HTTP request: %w", err)
	}

	token, err := utils.GetAccessTokenFromBackend(ctx)
	if err != nil {
		return "", err
	}

	// Set the Authorization header with the token from the request header
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
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

func DeleteFiles(ctx *gin.Context) {
	var body struct {
		ProjectID string `json:"project_id"`
		FileID    string `json:"file_id"`
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
	var projectName, userName, orgName string

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
	    p.id = $1;
	`, projectId).Scan(&userName, &projectName, &orgName)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"message": "Error getting project details from DB : " + err.Error(),
		})
		return
	}

	folderBase64, err := getFolderJsonFromGithub(ctx, projectName, userName, orgName, "")
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, err)
		return
	}
	// Step 1: Decode the Base64 string
	jsonBytes, err := base64.StdEncoding.DecodeString(folderBase64)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"message": "Error getting project details from DB : " + err.Error(),
		})
		return
	}

	// Step 2: Unmarshal the JSON into a Go struct
	var folder []models.Folder
	if err := json.Unmarshal(jsonBytes, &folder); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"message": "Error getting project details from DB : " + err.Error(),
		})
		return
	}

	updatedFolder, err := deleteFolderContents(ctx, folder, projectName, userName, fileID, orgName)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"message": "Error While deleting files : " + err.Error(),
		})
		return
	}

	// Check if updatedFolder is null, and if so, set it to an empty array
	if updatedFolder == nil {
		updatedFolder = []models.Folder{}
	}

	// Step 1: Marshal the JSON object to a JSON string
	jsonBytes, err = json.Marshal(updatedFolder)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"message": "Error While marshaling folder : " + err.Error(),
		})
		return
	}

	// Step 2: Encode the JSON string to Base64
	base64String := base64.StdEncoding.EncodeToString(jsonBytes)

	err = updateFolderStructure(ctx, userName, projectName, base64String, orgName)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, err)
		return
	}

	ctx.JSON(http.StatusOK, base64String)

}

func deleteFolderContents(ctx *gin.Context, folders []models.Folder, repoName string, repoOwner string, fileID uuid.UUID, org string) ([]models.Folder, error) {

	err := recusrsive(ctx, folders, repoName, repoOwner, fileID, org)
	if err != nil {
		return nil, err
	}

	updatedFolder, err := removeDeletedFolder(folders, fileID)
	if err != nil {
		return nil, err
	}

	return updatedFolder, nil
}

func removeDeletedFolder(folders []models.Folder, fileID uuid.UUID) ([]models.Folder, error) {
	var updatedFolder []models.Folder
	flag := false
	for _, folder := range folders {
		if folder.ID == fileID {
			flag = true
			continue
		}
		if len(folder.Children) > 0 && !flag {
			// Update the children and assign them back to the folder
			updatedChildren, err := removeDeletedFolder(folder.Children, fileID)
			if err != nil {
				return nil, err
			}
			// Check if the updated children is nil or empty
			if len(updatedChildren) == 0 {
				folder.Children = []models.Folder{} // Ensure it's set to an empty slice
			} else {
				folder.Children = updatedChildren
			}
		}
		updatedFolder = append(updatedFolder, folder)
	}

	return updatedFolder, nil
}

func recusrsive(ctx *gin.Context, folders []models.Folder, repoName string, repoOwner string, fileID uuid.UUID, org string) error {
	for _, folder := range folders {
		if len(folder.Children) > 0 {
			err := recusrsive(ctx, folder.Children, repoName, repoOwner, fileID, org)
			if err != nil {
				return fmt.Errorf("failed to delete child files: %w", err)
			}
		}
		if folder.ID == fileID {
			if len(folder.Children) > 0 {
				err := recDeleteFile(ctx, folder.Children, repoName, repoOwner, org)
				if err != nil {
					return fmt.Errorf("failed to delete child files: %w", err)
				}
			}
			if err := deleteFileFromGithub(ctx, repoName, repoOwner, fileID, org); err != nil {
				return err
			}
		}

	}
	return nil
}

func recDeleteFile(ctx *gin.Context, folders []models.Folder, repoName string, repoOwner string, org string) error {
	for _, folder := range folders {
		if len(folder.Children) > 0 {
			recDeleteFile(ctx, folder.Children, repoName, repoOwner, org)
		}
		err := deleteFileFromGithub(ctx, repoName, repoOwner, folder.ID, org)
		if err != nil {
			return err
		}
	}
	return nil
}

func deleteFileFromGithub(ctx *gin.Context, repoName string, repoOwner string, fileID uuid.UUID, org string) error {
	sha, err := getFileSha(ctx, repoName, repoOwner, fileID, org)
	if err != nil {
		return fmt.Errorf("failed to get sha of file: %w", err)
	}

	// Prepare the request body for GitHub API
	requestBody, err := json.Marshal(map[string]interface{}{
		"message": "deletes file content",
		"sha":     sha,
	})
	if err != nil {
		return err
	}

	// Create a new HTTP request to GitHub API
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/Documentthing/files/%s.json", repoOwner, repoName, fileID)

	if org != "" {
		url = fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/Documentthing/files/%s.json", org, repoName, fileID)
	}

	req, err := http.NewRequest("DELETE", url, bytes.NewBuffer(requestBody))
	if err != nil {
		return fmt.Errorf("failed to create new HTTP request: %w", err)
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
		return fmt.Errorf("failed to make HTTP request: %w", err)
	}
	defer resp.Body.Close()

	// Handle response from GitHub API
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to get repository: %s", resp.Status)
	}

	return nil
}

func UpdateFileName(ctx *gin.Context) {
	var body struct {
		ProjectID string `json:"project_id"`
		FileID    string `json:"file_id"`
		Name      string `json:"name"`
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
	var projectName, userName, orgName string

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
	    p.id = $1;
	`, projectId).Scan(&userName, &projectName, &orgName)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"message": "Error getting project details from DB : " + err.Error(),
		})
		return
	}

	folderBase64, err := getFolderJsonFromGithub(ctx, projectName, userName, orgName, "")
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, err)
		return
	}
	// Step 1: Decode the Base64 string
	jsonBytes, err := base64.StdEncoding.DecodeString(folderBase64)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"message": "Error getting project details from DB : " + err.Error(),
		})
		return
	}

	// Step 2: Unmarshal the JSON into a Go struct
	var folder []models.Folder
	if err := json.Unmarshal(jsonBytes, &folder); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"message": "Error getting project details from DB : " + err.Error(),
		})
		return
	}

	updatedFolder, err := updateFolderWithUpdatedFileName(folder, fileID, body.Name)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, err.Error())
		return
	}

	// Check if updatedFolder is null, and if so, set it to an empty array
	if updatedFolder == nil {
		updatedFolder = []models.Folder{}
	}

	// Step 1: Marshal the JSON object to a JSON string
	jsonBytes, err = json.Marshal(updatedFolder)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"message": "Error While marshaling folder : " + err.Error(),
		})
		return
	}

	// Step 2: Encode the JSON string to Base64
	base64String := base64.StdEncoding.EncodeToString(jsonBytes)

	err = updateFolderStructure(ctx, userName, projectName, base64String, orgName)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, err)
		return
	}

	ctx.JSON(http.StatusOK, base64String)
}

func updateFolderWithUpdatedFileName(folders []models.Folder, fileID uuid.UUID, name string) ([]models.Folder, error) {
	var updatedFolder []models.Folder

	for _, folder := range folders {
		if folder.ID == fileID {
			folder.Name = name
		}
		if len(folder.Children) > 0 {
			updatedChild, err := updateFolderWithUpdatedFileName(folder.Children, fileID, name)
			if err != nil {
				return nil, errors.New("error while updating the file name")
			}
			folder.Children = updatedChild
		}

		updatedFolder = append(updatedFolder, folder)
	}
	return updatedFolder, nil
}
