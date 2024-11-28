package controller

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"

	"github.com/Akshdhiwar/simpledocs-backend/internals/initializer"
	"github.com/Akshdhiwar/simpledocs-backend/internals/models"
	"github.com/Akshdhiwar/simpledocs-backend/internals/utils"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func GetFolder(ctx *gin.Context) {
	id := ctx.Param("id")
	t := ctx.Param("type")
	userID := ctx.GetHeader("X-User-Id")

	// parsing UUID for project id
	projectID, err := uuid.Parse(id)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"message": "Error parsing UUID" + err.Error(),
		})
		return
	}

	// getting details from DB
	var projectName, repoName, orgName string

	if t == "google" {
		err = initializer.DB.QueryRow(context.Background(), `SELECT
  			repo_owner,
  				NAME,
  				COALESCE(org, '') AS org
				FROM
  				projects
				WHERE
  			id = $1;`, projectID).Scan(&repoName, &projectName, &orgName)

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
		`, projectID, userID).Scan(&repoName, &projectName, &orgName)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{
				"message": "Error getting project details from DB : " + err.Error(),
			})
			return
		}
	}

	content, err := getFolderJsonFromGithub(ctx, projectName, repoName, orgName, t)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, err)
		return
	}

	ctx.JSON(http.StatusOK, content)
}

func getFolderJsonFromGithub(ctx *gin.Context, repoName string, userName string, org string, t string) (string, error) {

	// Create a new HTTP request to GitHub API
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/simpledocs/folder/folder.json", userName, repoName)

	if org != "" {
		url = fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/simpledocs/folder/folder.json", org, repoName)
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

	if resp.StatusCode == 401 {
		utils.GetNewAccessTokenFromGithub(ctx, repoName, t)
		// Re-run the function after refreshing the token
		return getFolderJsonFromGithub(ctx, repoName, userName, org, t)
	}

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
		ID       string        `json:"id"`
		ParentID string        `json:"parentID"`
		Folder   models.Folder `json:"folder"`
	}

	// Bind JSON request body to struct
	if err := ctx.ShouldBindJSON(&body); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request payload: " + err.Error()})
		return
	}

	// Parse project ID as UUID
	projectID, err := uuid.Parse(body.ID)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"message": "Invalid project ID format: " + err.Error()})
		return
	}

	var userName, repoName, org string

	// Query database for project and user information
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
	`, projectID).Scan(&userName, &repoName, &org)

	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": "Error querying the database: " + err.Error()})
		return
	}

	// Retrieve folder structure from GitHub
	folderBase64, err := getFolderJsonFromGithub(ctx, repoName, userName, org, "")
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": "Error retrieving folder structure from GitHub: " + err.Error()})
		return
	}

	// Decode Base64 folder structure
	jsonBytes, err := base64.StdEncoding.DecodeString(folderBase64)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": "Error decoding folder structure: " + err.Error()})
		return
	}

	// Unmarshal JSON into Go struct
	var folders []models.Folder
	if err := json.Unmarshal(jsonBytes, &folders); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": "Error unmarshaling folder structure: " + err.Error()})
		return
	}

	// Update folder structure based on parentID
	if body.ParentID == "" {
		folders = append(folders, models.Folder{ID: body.Folder.ID, Children: body.Folder.Children, Name: body.Folder.Name})
	} else {
		folders = recursiveAddFileInFolder(folders, body.ParentID, body.Folder)
	}

	// Marshal updated folder structure back to JSON
	jsonBytes, err = json.Marshal(folders)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": "Error marshaling updated folder structure: " + err.Error()})
		return
	}

	// Encode JSON string to Base64
	base64String := base64.StdEncoding.EncodeToString(jsonBytes)

	// Update folder structure on GitHub
	if err := updateFolderStructure(ctx, userName, repoName, base64String, org); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": "Error updating folder structure on GitHub: " + err.Error()})
		return
	}

	// Create a new file
	if err := createFile(ctx, userName, repoName, body.Folder.ID.String(), org); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": "Error creating file on GitHub: " + err.Error()})
		return
	}

	// Respond with the updated folder structure
	ctx.JSON(http.StatusCreated, folders)
}

// Recursive function to add a file into the correct folder
func recursiveAddFileInFolder(folders []models.Folder, parentID string, file models.Folder) []models.Folder {
	var updatedFolders []models.Folder

	for _, folder := range folders {
		if folder.ID.String() == parentID {
			// Add file to the children of the matched folder
			folder.Children = append(folder.Children, models.Folder{ID: file.ID, Children: file.Children, Name: file.Name})
		}
		// Recursively update children folders
		if len(folder.Children) > 0 {
			folder.Children = recursiveAddFileInFolder(folder.Children, parentID, file)
		}
		updatedFolders = append(updatedFolders, folder)
	}

	return updatedFolders
}

func createFile(ctx *gin.Context, userName string, repoName string, fileId string, org string) error {
	// Prepare the request body for GitHub API
	requestBody, err := json.Marshal(map[string]interface{}{
		"message": "created file " + fileId,
		"content": "IntcIjNkNGIxN2QwLTlmODUtNDViOC1iOGI1LWM5M2M0MGFmNTE3ZlwiOntcImlkXCI6XCIzZDRiMTdkMC05Zjg1LTQ1YjgtYjhiNS1jOTNjNDBhZjUxN2ZcIixcInZhbHVlXCI6W3tcImNoaWxkcmVuXCI6W3tcInRleHRcIjpcImltcG9ydCB7IHBhc3Npb24sIHBlcnNldmVyYW5jZSB9IGZyb20gJ2xpZmUnO1xcblxcbndoaWxlICh0cnVlKSB7XFxuICAgIGRyZWFtKCk7XFxuICAgIGNvZGUoKTtcXG4gICAgaW1wcm92ZSgpO1xcbn1cIn1dLFwidHlwZVwiOlwiY29kZVwiLFwiaWRcIjpcIjJkMWI1OTIwLWZlNTAtNGJhNi05NTcwLTk5ZDk1ZjhhZDNjNlwiLFwicHJvcHNcIjp7XCJsYW5ndWFnZVwiOlwiSmF2YVNjcmlwdFwiLFwidGhlbWVcIjpcIlZTQ29kZVwiLFwibm9kZVR5cGVcIjpcInZvaWRcIn19XSxcInR5cGVcIjpcIkNvZGVcIixcIm1ldGFcIjp7XCJvcmRlclwiOjEsXCJkZXB0aFwiOjB9fSxcIjQ3ODNkYTg5LWY5NGItNDNjZS1iYzkwLTdiODNkYjJiMWMxNlwiOntcImlkXCI6XCI0NzgzZGE4OS1mOTRiLTQzY2UtYmM5MC03YjgzZGIyYjFjMTZcIixcInZhbHVlXCI6W3tcImlkXCI6XCJjZWFiZTdiMS00ZjE2LTQ0NWEtOWM0Yi1mMWExMWNiNWRiNWVcIixcInR5cGVcIjpcImhlYWRpbmctb25lXCIsXCJjaGlsZHJlblwiOlt7XCJ0ZXh0XCI6XCJIZWxsbyBuZXcgZmlsZSBjcmVhdGVkXCJ9XSxcInByb3BzXCI6e1wibm9kZVR5cGVcIjpcImJsb2NrXCJ9fV0sXCJ0eXBlXCI6XCJIZWFkaW5nT25lXCIsXCJtZXRhXCI6e1wib3JkZXJcIjowLFwiZGVwdGhcIjowfX0sXCI3YjJmYzhmZS02ZWUwLTQ4OTItODBkNC1mMjkyMzEzMjU3YTdcIjp7XCJpZFwiOlwiN2IyZmM4ZmUtNmVlMC00ODkyLTgwZDQtZjI5MjMxMzI1N2E3XCIsXCJ2YWx1ZVwiOlt7XCJpZFwiOlwiYjM3ZjZkYzYtMDg5NC00ZjlkLWI4NTEtMWI4YTY1NTUxMjQ3XCIsXCJ0eXBlXCI6XCJibG9ja3F1b3RlXCIsXCJjaGlsZHJlblwiOlt7XCJib2xkXCI6dHJ1ZSxcInRleHRcIjpcIi0gT3VyIGxpZmUgaXMgd2hhdCBvdXIgdGhvdWdodHMgbWFrZSBpdFwifSx7XCJ0ZXh0XCI6XCIgKGMpIE1hcmN1cyBBdXJlbGl1c1wifV0sXCJwcm9wc1wiOntcIm5vZGVUeXBlXCI6XCJibG9ja1wifX1dLFwidHlwZVwiOlwiQmxvY2txdW90ZVwiLFwibWV0YVwiOntcIm9yZGVyXCI6MixcImRlcHRoXCI6MH19LFwiNzZiMzQ5NGQtYzhmMy00OGVhLThhODAtMjA5YThiNzI2MzRiXCI6e1wiaWRcIjpcIjc2YjM0OTRkLWM4ZjMtNDhlYS04YTgwLTIwOWE4YjcyNjM0YlwiLFwidmFsdWVcIjpbe1wiaWRcIjpcIjM3NjZmMzU4LTEzMzQtNGQ1OC05NjhlLWNiMzU0NjI0NzMwMlwiLFwidHlwZVwiOlwicGFyYWdyYXBoXCIsXCJjaGlsZHJlblwiOlt7XCJ0ZXh0XCI6XCJcIn1dLFwicHJvcHNcIjp7XCJub2RlVHlwZVwiOlwiYmxvY2tcIn19XSxcInR5cGVcIjpcIlBhcmFncmFwaFwiLFwibWV0YVwiOntcIm9yZGVyXCI6MyxcImRlcHRoXCI6MH19LFwiZDYyODg2NGUtYTY2NS00NmVjLWExNDQtMmI5MzZiNDU4Zjk0XCI6e1wiaWRcIjpcImQ2Mjg4NjRlLWE2NjUtNDZlYy1hMTQ0LTJiOTM2YjQ1OGY5NFwiLFwidmFsdWVcIjpbe1wiaWRcIjpcIjMyZGVlZDdhLTRiYzMtNDk5Yy04ZGQ2LTg3NjdmYzVkZTRmNVwiLFwidHlwZVwiOlwicGFyYWdyYXBoXCIsXCJjaGlsZHJlblwiOlt7XCJ0ZXh0XCI6XCJcIn1dLFwicHJvcHNcIjp7XCJub2RlVHlwZVwiOlwiYmxvY2tcIn19XSxcInR5cGVcIjpcIlBhcmFncmFwaFwiLFwibWV0YVwiOntcIm9yZGVyXCI6NCxcImRlcHRoXCI6MH19fSI=", // Base64-encoded empty string for folder creation
	})
	if err != nil {
		return err
	}

	// The URL should point to the desired folder path, using an empty file name to create the folder
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/simpledocs/files/%s.json", userName, repoName, fileId)
	if org != "" {
		url = fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/simpledocs/files/%s.json", org, repoName, fileId)
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
		return errors.New("failed to create a file in repository")
	}

	return nil
}

func updateFolderStructure(ctx *gin.Context, userName string, repoName string, content string, org string) error {

	// Get the latest SHA for the folder
	sha, err := getFolderSHA(ctx, repoName, userName, org)
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
	if org != "" {
		url = fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/simpledocs/folder/folder.json", org, repoName)
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
	if resp.StatusCode != http.StatusOK {
		return errors.New("failed to update folder in repository")
	}

	return nil
}

func getFolderSHA(ctx *gin.Context, repoName string, userName string, org string) (string, error) {

	// Create a new HTTP request to GitHub API
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/simpledocs/folder/folder.json", userName, repoName)

	if org != "" {
		url = fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/simpledocs/folder/folder.json", org, repoName)
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

func GetAccessTokenFromBackendTypeGoogle(ctx *gin.Context, t string, repoName string) (string, error) {

	id := ctx.GetHeader("X-User-Id")
	var encryptedToken, name string
	var githubID int

	if t == "google" {
		err := initializer.DB.QueryRow(context.Background(), `
			SELECT
		u.id
	  FROM
		public.projects p
		JOIN public.users u ON p.owner = u.id
	  WHERE
		p.name = $1;
		`, repoName).Scan(&id)

		if err == sql.ErrNoRows {
			return "", fmt.Errorf("project not found")
		} else if err != nil {
			return "", err
		}
	}

	err := initializer.DB.QueryRow(context.Background(), `SELECT token , github_name , github_id from users WHERE id = $1`, id).Scan(&encryptedToken, &name, &githubID)
	if err != nil {
		return "", err
	}

	key := utils.DeriveKey(id + os.Getenv("ENC_SECRET"))

	token, err := utils.Decrypt(encryptedToken, key)
	if err != nil {
		return "", err
	}

	return token, nil
}
