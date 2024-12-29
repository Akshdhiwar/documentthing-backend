package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Akshdhiwar/simpledocs-backend/internals/initializer"
	"github.com/Akshdhiwar/simpledocs-backend/internals/utils"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func CommitChanges(ctx *gin.Context) {

	var body struct {
		ProjectID string     `json:"project_id"`
		Content   []Contents `json:"content"`
		Message   string     `json:"message"`
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

	latestCommistSha, err := getLatestShaFromGithub(ctx, projectName, userName, org, "main")
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, err.Error())
		return
	}

	latestCommitTreeSha, err := getLatestTreeShaForCommit(ctx, projectName, userName, org, latestCommistSha)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, err.Error())
		return
	}

	latestTreeSha, err := createNewTreeForCommit(ctx, projectName, userName, org, latestCommitTreeSha, body.Content)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, err.Error())
		return
	}

	newCommitSha, err := createNewCommit(ctx, projectName, userName, org, latestTreeSha, latestCommistSha, body.Message)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, err.Error())
		return
	}

	err = updateReferenceToNewCommit(ctx, projectName, userName, org, newCommitSha, "main")
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, err.Error())
		return
	}

	utils.NotifyUsers(body.ProjectID, ctx.GetHeader("X-User-Id"))

	ctx.JSON(http.StatusOK, gin.H{
		"message": "Changes committed successfully",
	})

}

// function to get latest commit sha from github

func getLatestShaFromGithub(ctx *gin.Context, repoName, userName, org, branchName string) (string, error) {
	// Create a new HTTP request to GitHub API
	// https://api.github.com/repos/username/repo/git/ref/heads/main
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/git/ref/heads/%s", userName, repoName, branchName)

	if org != "" {
		url = fmt.Sprintf("https://api.github.com/repos/%s/%s/git/ref/heads/%s", org, repoName, branchName)
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
	var githubResp RefResponse
	if err := json.NewDecoder(resp.Body).Decode(&githubResp); err != nil {
		return "", fmt.Errorf("failed to decode response body: %w", err)
	}

	if githubResp.Object.SHA == "" {
		return "", fmt.Errorf("failed to get latest commit SHA")
	}

	// Return the content and SHA in a map
	return githubResp.Object.SHA, nil
}

// get the latest sha tree for that commit
func getLatestTreeShaForCommit(ctx *gin.Context, repoName string, userName string, org string, commitSHA string) (string, error) {
	// Create a new HTTP request to GitHub API
	// https://api.github.com/repos/username/repo/git/ref/heads/main
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/git/commits/%s", userName, repoName, commitSHA)

	if org != "" {
		url = fmt.Sprintf("https://api.github.com/repos/%s/%s/git/commits/%s", org, repoName, commitSHA)
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
	var githubResp CommitResponse
	if err := json.NewDecoder(resp.Body).Decode(&githubResp); err != nil {
		return "", fmt.Errorf("failed to decode response body: %w", err)
	}

	if githubResp.Tree.Sha == "" {
		return "", fmt.Errorf("failed to get latest tree SHA")
	}

	// Return the content and SHA in a map
	return githubResp.Tree.Sha, nil
}

type AddOrUpdateFile struct {
	Path    string `json:"path"`
	Mode    string `json:"mode"`
	Type    string `json:"type"`
	Content string `json:"content"`
}

func createNewTreeForCommit(ctx *gin.Context, repoName string, userName string, org string, latestTreeSha string, content []Contents) (string, error) {

	type DeleteFile struct {
		Path string      `json:"path"`
		Mode string      `json:"mode"`
		Type string      `json:"type"`
		Sha  interface{} `json:"sha"`
	}

	var blobContents []interface{}

	for _, c := range content {
		if c.ChangedContent == "null" {
			// File to be deleted
			deleteFile := DeleteFile{
				Path: c.Path,
				Mode: "100644",
				Type: "blob",
				Sha:  nil,
			}
			blobContents = append(blobContents, deleteFile)
		} else {
			// File to be added or modified
			addOrUpdateFile := AddOrUpdateFile{
				Path:    c.Path,
				Mode:    "100644",
				Type:    "blob",
				Content: c.ChangedContent,
			}
			blobContents = append(blobContents, addOrUpdateFile)
		}
	}

	requestBody, err := json.Marshal(map[string]interface{}{
		"base_tree": latestTreeSha,
		"tree":      blobContents,
	})

	if err != nil {
		return "", err
	}
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/git/trees", userName, repoName)
	if org != "" {
		url = fmt.Sprintf("https://api.github.com/repos/%s/%s/git/trees", org, repoName)
	}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(requestBody))
	if err != nil {
		return "", fmt.Errorf("failed to create new HTTP request: %w", err)
	}

	token, err := utils.GetAccessTokenFromBackend(ctx)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to make HTTP request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("failed to create tree: %s", resp.Status)
	}
	var githubResp TreeResponse
	if err := json.NewDecoder(resp.Body).Decode(&githubResp); err != nil {
		return "", fmt.Errorf("failed to decode response body: %w", err)
	}
	return githubResp.Sha, nil

}

func createNewCommit(ctx *gin.Context, repoName string, userName string, org string, latestTreeSha string, lastCommitSha string, message string) (string, error) {
	requestBody, err := json.Marshal(map[string]interface{}{
		"message": message,
		"tree":    latestTreeSha,
		"parents": []string{lastCommitSha},
	})
	if err != nil {
		return "", err
	}
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/git/commits", userName, repoName)
	if org != "" {
		url = fmt.Sprintf("https://api.github.com/repos/%s/%s/git/commits", org, repoName)
	}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(requestBody))
	if err != nil {
		return "", err
	}
	token, err := utils.GetAccessTokenFromBackend(ctx)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("failed to create commit: %s", resp.Status)
	}
	var githubResp CommitResponse
	if err := json.NewDecoder(resp.Body).Decode(&githubResp); err != nil {
		return "", fmt.Errorf("failed to decode response body: %w", err)
	}
	return githubResp.Sha, nil
}

func updateReferenceToNewCommit(ctx *gin.Context, repoName string, userName string, org string, latestCommitSha string, branchName string) error {

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/git/refs/heads/%s", userName, repoName, branchName)
	if org != "" {
		url = fmt.Sprintf("https://api.github.com/repos/%s/%s/git/refs/heads/%s", org, repoName, branchName)
	}
	requestBody, err := json.Marshal(map[string]string{
		"sha": latestCommitSha,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	req, err := http.NewRequest("PATCH", url, bytes.NewBuffer(requestBody))
	if err != nil {
		return fmt.Errorf("failed to create new HTTP request: %w", err)
	}
	token, err := utils.GetAccessTokenFromBackend(ctx)
	if err != nil {
		return fmt.Errorf("failed to get access token: %w", err)
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make HTTP request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to update reference: %s", resp.Status)
	}
	return nil
}

// RefResponse represents the response structure
type RefResponse struct {
	Ref    string `json:"ref"`
	NodeID string `json:"node_id"`
	URL    string `json:"url"`
	Object struct {
		SHA  string `json:"sha"`
		Type string `json:"type"`
		URL  string `json:"url"`
	} `json:"object"`
}

type CommitResponse struct {
	Sha          string       `json:"sha"`
	NodeID       string       `json:"node_id"`
	URL          string       `json:"url"`
	HtmlURL      string       `json:"html_url"`
	Author       User         `json:"author"`
	Committer    User         `json:"committer"`
	Tree         Tree         `json:"tree"`
	Message      string       `json:"message"`
	Parents      []Parent     `json:"parents"`
	Verification Verification `json:"verification"`
}

type BlobContent struct {
	Path    string  `json:"path"`
	Mode    string  `json:"mode"`
	Type    string  `json:"type"`
	Content string  `json:"content,omitempty"`
	Sha     *string `json:"sha,omitempty"`
}

type User struct {
	Name  string    `json:"name"`
	Email string    `json:"email"`
	Date  time.Time `json:"date"`
}

type Tree struct {
	Sha string `json:"sha"`
	URL string `json:"url"`
}

type Parent struct {
	Sha     string `json:"sha"`
	URL     string `json:"url"`
	HtmlURL string `json:"html_url"`
}

type Verification struct {
	Verified  bool    `json:"verified"`
	Reason    string  `json:"reason"`
	Signature *string `json:"signature"`
	Payload   *string `json:"payload"`
}

type Contents struct {
	Type            string `json:"type"`
	Path            string `json:"path"`
	Name            string `json:"name"`
	Id              string `json:"id"`
	OriginalContent string `json:"originalContent"`
	ChangedContent  string `json:"changedContent"`
}

type TreeResponse struct {
	Sha       string      `json:"sha"`
	URL       string      `json:"url"`
	Tree      []TreeEntry `json:"tree"`
	Truncated bool        `json:"truncated"`
}

type TreeEntry struct {
	Path string `json:"path"`
	Mode string `json:"mode"`
	Type string `json:"type"`
	Sha  string `json:"sha"`
	Size *int   `json:"size,omitempty"` // Size is only present for blobs, so it is optional
	URL  string `json:"url"`
}

func CommitEditingChanges(ctx *gin.Context) {
	var body struct {
		ProjectID  string     `json:"project_id"`
		Content    []Contents `json:"content"`
		Message    string     `json:"message"`
		BranchName string     `json:"branch_name"`
		PR         bool       `json:"pr"`
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

	latestCommistSha, err := getLatestShaFromGithub(ctx, projectName, userName, org, body.BranchName)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, err.Error())
		return
	}

	latestCommitTreeSha, err := getLatestTreeShaForCommit(ctx, projectName, userName, org, latestCommistSha)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, err.Error())
		return
	}

	latestTreeSha, err := createNewTreeForCommit(ctx, projectName, userName, org, latestCommitTreeSha, body.Content)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, err.Error())
		return
	}

	newCommitSha, err := createNewCommit(ctx, projectName, userName, org, latestTreeSha, latestCommistSha, body.Message)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, err.Error())
		return
	}

	err = updateReferenceToNewCommit(ctx, projectName, userName, org, newCommitSha, body.BranchName)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, err.Error())
		return
	}

	if body.PR {
		err := CreatePullRequest(ctx, userName, org, projectName, body.BranchName, body.BranchName)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, err.Error())
			return
		}
		delete(EditingBranchesMappings, projectId.String()+userID)
	}

	ctx.JSON(http.StatusOK, gin.H{
		"message": "Changes committed successfully",
	})

}

// PullRequest represents the payload for creating a pull request
type PullRequest struct {
	Title string `json:"title"`
	Head  string `json:"head"`
	Base  string `json:"base"`
	Body  string `json:"body"`
}

// CreatePullRequest creates a pull request using the GitHub API.
func CreatePullRequest(ctx *gin.Context, owner, org, repo, headBranch, title string) error {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls", owner, repo)

	if org != "" {
		url = fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls", org, repo)
	}

	// Construct the pull request payload
	pr := PullRequest{
		Title: title,
		Head:  headBranch,
		Base:  "main",
	}

	// Serialize the payload to JSON
	payload, err := json.Marshal(pr)
	if err != nil {
		return fmt.Errorf("failed to serialize pull request payload: %w", err)
	}

	// Create an HTTP client and request
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	token, err := utils.GetAccessTokenFromBackend(ctx)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("Content-Type", "application/json")

	// Send the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send HTTP request: %w", err)
	}
	defer resp.Body.Close()

	// Check for errors in the response
	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("failed to create pull request: status code %d", resp.StatusCode)
	}

	fmt.Println("Pull request created successfully")
	return nil
}

func SaveDrawings(ctx *gin.Context) {
	var body struct {
		ProjectID string `json:"project_id"`
		Content   string `json:"content"`
		Name      string `json:"name"`
		Message   string `json:"message"`
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

}
