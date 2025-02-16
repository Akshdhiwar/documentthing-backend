package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/Akshdhiwar/simpledocs-backend/internals/initializer"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func GetPublicFolder(ctx *gin.Context) {
	var name = ctx.Param("name")

	bucket := "public-docs"
	path := name + "/folder.json"

	// Get Object
	result, err := initializer.R2Client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(path),
	})
	if err != nil {
		log.Fatalf("Failed to get object: %v", err)
	}
	defer result.Body.Close()

	// Read the object data
	data, err := io.ReadAll(result.Body)
	if err != nil {
		log.Fatalf("Failed to read object data: %v", err)
	}

	ctx.JSON(http.StatusOK, string(data))
}

func GetPublicFile(ctx *gin.Context) {
	var name = ctx.Param("name")
	var id = ctx.Param("id")

	bucket := "public-docs"
	path := name + "/" + id + ".json"

	// Get Object
	result, err := initializer.R2Client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(path),
	})
	if err != nil {
		log.Fatalf("Failed to get object: %v", err)
	}
	defer result.Body.Close()

	// Read the object data
	data, err := io.ReadAll(result.Body)
	if err != nil {
		log.Fatalf("Failed to read object data: %v", err)
	}

	ctx.JSON(http.StatusOK, string(data))

}

func PublishDocs(ctx *gin.Context) {
	var body struct {
		ProjectID string `json:"project_id"`
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

	contents, err := getAllContents(ctx, projectName, userName, org, "github")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	uploadFiles(contents, strings.ToLower(projectName))

	_, err = initializer.DB.Exec(context.Background(), `
	UPDATE public.projects
	SET is_published = $1, published_docs_name = $2
	WHERE id = $3;
	`, true, strings.ToLower(projectName), projectId)

	if err != nil {
		log.Println("Error updating project:", err)
	}

	ctx.Status(http.StatusOK)

}

func getFolderAndFilesJsonFormGithub(ctx *gin.Context, repoName, userName, org, t, path string) ([]GitHubContent, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s", userName, repoName, path)

	if org != "" {
		url = fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s", org, repoName, path)
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create new HTTP request: %w", err)
	}

	token, err := GetAccessTokenFromBackendTypeGoogle(ctx, t, repoName)
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

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var contents []GitHubContent
	err = json.Unmarshal(body, &contents)
	if err != nil {
		return nil, err
	}

	return contents, nil
}

func getAllContents(ctx *gin.Context, repoName, userName, org, t string) ([]FileContent, error) {
	allContents := []GitHubContent{}

	// Fetch Documentthing contents
	contents, err := getFolderAndFilesJsonFormGithub(ctx, repoName, userName, org, t, "Documentthing")
	if err != nil {
		return nil, err
	}

	// Check for "files" and "folder" directories
	for _, item := range contents {
		if item.Name == "files" || item.Name == "folder" {
			subContents, err := getFolderAndFilesJsonFormGithub(ctx, repoName, userName, org, t, item.Path)
			if err != nil {
				return nil, err
			}
			allContents = append(allContents, subContents...)
		}
	}

	token, err := GetAccessTokenFromBackendTypeGoogle(ctx, t, repoName)
	if err != nil {
		return nil, err
	}

	var r2Contents []FileContent

	// Fetch contents of individual files
	for _, item := range allContents {
		if item.Type == "file" {
			content, err := fetchFileContent(item.Url, token)
			if err != nil {
				return nil, err
			}

			// Store the content and path in r2Contents
			r2Contents = append(r2Contents, FileContent{
				Path:    item.Name, // Capitalize 'Path' and 'Content' for proper struct initialization
				Content: content,
			})
		}
	}

	return r2Contents, nil
}

type GitHubContent struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Path string `json:"path"`
	Url  string `json:"url"`
}

func fetchFileContent(url, token string) (string, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create new HTTP request: %w", err)
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

	// Return the content
	return githubResp.Content, nil
}

func uploadFiles(contents []FileContent, projectName string) {
	var wg sync.WaitGroup

	bucket := "public-docs"

	// Upload each file in a separate Goroutine
	for _, file := range contents {
		wg.Add(1)
		go func(file FileContent) {
			defer wg.Done()

			// Convert the content string to a byte slice
			fileContent := []byte(file.Content)
			path := projectName + "/" + file.Path
			// Upload to R2 (Cloudflare S3-compatible storage)
			_, err := initializer.R2Client.PutObject(context.TODO(), &s3.PutObjectInput{
				Bucket: &bucket,
				Key:    &path, // Use the file path as the object key
				Body:   bytes.NewReader(fileContent),
			})
			if err != nil {
				log.Printf("Failed to upload %s: %v", file.Path, err)
				return
			}
			fmt.Println("Uploaded:", file.Path)
		}(file)
	}

	// Wait for all uploads to finish
	wg.Wait()
	fmt.Println("All files uploaded successfully!")
}

type FileContent struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}
