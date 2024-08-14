package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/Akshdhiwar/simpledocs-backend/internals/initializer"
	"github.com/Akshdhiwar/simpledocs-backend/internals/models"
	"github.com/gin-gonic/gin"
)

func GetAccessTokenFromGithub(ctx *gin.Context) {

	// body creation to get code from payload

	var body struct {
		Code string `json:"code"`
	}

	err := ctx.ShouldBindJSON(&body)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, "Error binding data :"+err.Error())
		return
	}

	token, err := getAccessToken(body.Code)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, err.Error())
		return
	}

	ctx.JSON(http.StatusOK, token)

}

func GetUserDetailsFromGithub(ctx *gin.Context) {
	req, err := http.NewRequest("GET", "https://api.github.com/user", nil)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, "Error while creating request to github :"+err.Error())
		return
	}

	// Set the Authorization header with the access token
	req.Header.Set("Authorization", ctx.GetHeader("Authorization"))

	// Send the request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, "Request failed:"+err.Error())
		return
	}
	defer resp.Body.Close() // Ensure the response body is closed

	// Read the response body
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, "Failed to read response body:"+err.Error())
	}

	var userDetails map[string]interface{}

	err = json.Unmarshal(body, &userDetails)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, "Error while Unmarshalling")
		return
	}

	// Check if the status is 400 and handle it
	if resp.StatusCode == 400 {
		if message, exists := userDetails["message"].(string); exists {
			ctx.JSON(400, message)
		} else {
			ctx.JSON(400, "Bad request")
		}
		return
	}

	id := userDetails["id"].(float64)

	var exists bool
	err = initializer.DB.QueryRow(context.Background(), "SELECT EXISTS(SELECT 1 FROM users WHERE github_id = $1)", id).Scan(&exists)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"message": err.Error(),
			"type":    "error",
		})
		return
	}

	var user models.Users

	if !exists {

		if avatarURL, ok := userDetails["avatar_url"].(string); ok {
			user.AvatarURL = avatarURL
		}
		if company, ok := userDetails["company"].(string); ok {
			user.Company = company
		}
		if email, ok := userDetails["email"].(string); ok {
			user.Email = email
		}
		if twitter, ok := userDetails["twitter_username"].(string); ok {
			user.Twitter = twitter
		}
		user.GithubID = int(id)
		if githubName, ok := userDetails["login"].(string); ok {
			user.GithubName = githubName
		}
		if name, ok := userDetails["name"].(string); ok {
			user.Name = name
		}

		err := initializer.DB.QueryRow(context.Background(),
			`INSERT INTO users (avatar_url, company, email, twitter, github_id, github_name, name)
     			VALUES ($1, $2, $3, $4, $5, $6, $7)
     			RETURNING id, avatar_url, company, email, twitter, github_id, github_name, name`,
			user.AvatarURL, user.Company, user.Email, user.Twitter, user.GithubID, user.GithubName, user.Name).
			Scan(&user.ID, &user.AvatarURL, &user.Company, &user.Email, &user.Twitter, &user.GithubID, &user.GithubName, &user.Name)

		if err != nil {
			ctx.JSON(http.StatusInternalServerError, "Unable to save data to DB while creating user :"+err.Error())
			return
		}

		ctx.JSON(http.StatusOK, gin.H{
			"userDetails": user,
		})

		return
	}

	err = initializer.DB.QueryRow(context.Background(),
		`SELECT id , avatar_url , company , email , twitter , github_id , github_name , name FROM users WHERE github_id = $1`, id).
		Scan(&user.ID, &user.AvatarURL, &user.Company, &user.Email, &user.Twitter, &user.GithubID, &user.GithubName, &user.Name)

	if err != nil {
		ctx.JSON(http.StatusInternalServerError, "Unable to Get user :"+err.Error())
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"userDetails": user,
	})

}

func getAccessToken(code string) (any, error) {
	clientID := os.Getenv("RAILS_GITHUB_APP_ID")
	clientSecret := os.Getenv("RAILS_GITHUB_APP_SECRET")

	// Set up the request body as JSON
	requestBodyMap := map[string]string{
		"client_id":     clientID,
		"client_secret": clientSecret,
		"code":          code,
	}
	requestJSON, err := json.Marshal(requestBodyMap)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request body: %w", err)
	}

	// Create the POST request
	req, err := http.NewRequest(
		"POST",
		"https://github.com/login/oauth/access_token",
		bytes.NewBuffer(requestJSON),
	)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Send the request and get the response
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read and parse the response body
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	var token any
	err = json.Unmarshal(respBody, &token)
	if err != nil {
		return "", fmt.Errorf("failed to unmarshal response body: %w", err)
	}

	return token, nil
}
