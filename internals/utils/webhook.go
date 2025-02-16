package utils

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
	"sync"

	"github.com/Akshdhiwar/simpledocs-backend/internals/initializer"
	"github.com/Akshdhiwar/simpledocs-backend/internals/models"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gin-gonic/gin"
)

func HandleWebhookEvents(ctx *gin.Context) {
	// Log that the webhook was called
	fmt.Println("Webhook received")

	// Step 1: Parse the webhook event payload
	var event PayPalWebhookEvent
	if err := ctx.ShouldBindJSON(&event); err != nil {
		fmt.Println("Error parsing webhook payload:", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload"})
		return
	}

	// Step 2: Handle different event types based on EventType
	switch event.EventType {
	case "BILLING.SUBSCRIPTION.CREATED":
		var resource models.Resource
		if err := json.Unmarshal(event.Resource, &resource); err == nil {
			fmt.Println("Subscription Created:", resource.ID)
			// fmt.Println("Subscription Created:", event)
			// Perform actions such as logging or storing in the database
		}

	case "BILLING.SUBSCRIPTION.ACTIVATED":
		var resource SubscriptionResource
		if err := json.Unmarshal(event.Resource, &resource); err == nil {
			fmt.Println("Subscription Activated:", resource.ID)
			// fmt.Println("Subscription Activated:", event)
			// Update user subscription status in database

			_, err := initializer.DB.Query(context.Background(),
				`UPDATE organizations SET status = true WHERE subscription_id = $1`,
				resource.ID)

			if err != nil {
				ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Error while updating subscription status"})
				return
			}
		}

	case "BILLING.SUBSCRIPTION.CANCELLED":
		var resource SubscriptionResource
		if err := json.Unmarshal(event.Resource, &resource); err == nil {
			fmt.Println("Subscription Cancelled:", resource.ID)
			// Update user subscription status in database
			_, err := initializer.DB.Query(context.Background(), `UPDATE organizations SET status = false WHERE subscription_id = $1 `, resource.ID)

			if err != nil {
				ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Error while updating subscription status"})
				return
			}
		}

	case "BILLING.SUBSCRIPTION.EXPIRED":
		var resource SubscriptionResource
		if err := json.Unmarshal(event.Resource, &resource); err == nil {
			fmt.Println("Subscription Expired:", resource.ID)
			// Update user subscription status in database
			_, err := initializer.DB.Query(context.Background(), `UPDATE organizations SET status = false WHERE subscription_id = $1 `, resource.ID)

			if err != nil {
				ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Error while updating subscription status"})
				return
			}
		}
	case "BILLING.SUBSCRIPTION.SUSPENDED":
		var resource SubscriptionResource
		if err := json.Unmarshal(event.Resource, &resource); err == nil {
			fmt.Println("Subscription Suspended:", resource.ID)
			// Update user subscription status in database
			_, err := initializer.DB.Query(context.Background(), `UPDATE organizations SET status = false WHERE subscription_id = $1 `, resource.ID)

			if err != nil {
				ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Error while updating subscription status"})
				return
			}
		}

	// Add other cases as necessary for events like BILLING.SUBSCRIPTION.RENEWED, PAYMENT.SALE.COMPLETED, etc.
	default:
		fmt.Println("Unhandled event type:", event.EventType)
	}

	// Respond with 200 OK to acknowledge receipt of the event
	ctx.JSON(http.StatusOK, gin.H{"message": "Webhook event received"})
}

// Define the general structure of the webhook event
type PayPalWebhookEvent struct {
	EventType string          `json:"event_type"`
	Resource  json.RawMessage `json:"resource"`
}

// Define specific event resources you want to handle (e.g., subscription details)
type SubscriptionResource struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	// Add additional fields if needed
}

type Commit struct {
	Modified []string `json:"modified"`
}

type WebhookPayload struct {
	Ref        string `json:"ref"`
	Repository struct {
		Name string `json:"name"`
	} `json:"repository"`
	Pusher struct {
		Username string `json:"name"`
	} `json:"pusher"`
	HeadCommit struct {
		Added    []string `json:"added"`
		Modified []string `json:"modified"`
		Removed  []string `json:"removed"`
	} `json:"head_commit"`
}

func HandleGithubWebhook(c *gin.Context) {
	var payload WebhookPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Error parsing JSON"})
		return
	}

	if payload.Ref == "refs/heads/main" || payload.Ref == "refs/heads/master" {
		userName := payload.Pusher.Username
		repoName := payload.Repository.Name

		isPublished := false

		err := initializer.DB.QueryRow(context.Background(), `
	SELECT is_published
	FROM public.projects
	WHERE name = $1;
		`, repoName).Scan(&isPublished)

		if err != nil {
			fmt.Println("Error while getting data from DB")
			return
		}

		if !isPublished {
			c.Status(http.StatusOK)
			return
		}

		allChangedFiles := append(payload.HeadCommit.Added, payload.HeadCommit.Modified...)
		filteredChangedFiles := filterJSONFiles(allChangedFiles)
		addedModifiedURLs := generateGitHubURLs(userName, repoName, filteredChangedFiles)
		// filteredRemovedFiles := filterJSONFiles(payload.HeadCommit.Removed)

		token, err := getTokenFromName(userName, repoName)
		if err != nil {
			fmt.Println("Failed to publish updated docs")
			return
		}

		var r2Contents []FileContent

		// Fetch contents of individual files
		for _, item := range addedModifiedURLs {
			content, err := fetchFileContent(item, token)
			if err != nil {
				fmt.Println("Failed to publish updated docs")
				return
			}

			// Store the content and path in r2Contents
			r2Contents = append(r2Contents, FileContent{
				Path:    extractFilenameFromURL(item), // Capitalize 'Path' and 'Content' for proper struct initialization
				Content: content,
			})
		}

		uploadFiles(r2Contents, strings.ToLower(repoName))

		c.Status(http.StatusOK)
		return
	}
	c.Status(http.StatusOK)
}

func extractFilenameFromURL(url string) string {
	// Extract the last part of the URL (the filename)
	filename := path.Base(url)
	// Ensure it ends with .json
	if strings.HasSuffix(filename, ".json") {
		return filename
	}
	return ""
}

func filterJSONFiles(files []string) []string {
	var jsonFiles []string
	for _, file := range files {
		if strings.HasSuffix(file, ".json") {
			jsonFiles = append(jsonFiles, file)
		}
	}
	return jsonFiles
}

func getTokenFromName(githubName, projectName string) (string, error) {

	var encryptedToken, id string

	err := initializer.DB.QueryRow(context.Background(), `
	SELECT u.id AS user_id, u.token
	FROM users u
	JOIN public.user_project_mapping upm ON u.id = upm.user_id
	JOIN public.projects p ON upm.project_id = p.id
	WHERE u.github_name = $1 AND p.name = $2;
	`, githubName, projectName).Scan(&id, &encryptedToken)
	if err != nil {
		return "", err
	}

	key := DeriveKey(id + os.Getenv("ENC_SECRET"))

	token, err := Decrypt(encryptedToken, key)
	if err != nil {
		return "", err
	}

	return token, nil
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

func generateGitHubURLs(username, repo string, files []string) []string {
	var urls []string
	for _, file := range files {
		url := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s", username, repo, file)
		urls = append(urls, url)
	}
	return urls
}
