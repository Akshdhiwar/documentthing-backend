package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/Akshdhiwar/simpledocs-backend/internals/initializer"
	"github.com/Akshdhiwar/simpledocs-backend/internals/models"
	"github.com/Akshdhiwar/simpledocs-backend/internals/utils"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/markbates/goth/gothic"
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

	userDetails, err := GetUserDetailsFromGithub(token)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, err.Error())
		return
	}

	key := utils.DeriveKey(userDetails.GithubName + os.Getenv("ENC_SECRET") + string(userDetails.GithubID))

	encToken, err := utils.Encrypt([]byte(token), key)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, "Error while encrypting the token")
		return
	}

	_, err = initializer.DB.Exec(context.Background(), "UPDATE users SET github_token = $1 WHERE id = $2", encToken, userDetails.ID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, "Error while updating encrypted token in users table")
		return
	}

	userAccessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"githubName": userDetails.GithubName,
		"email":      userDetails.Email,
		"sub":        userDetails.ID,
		"exp":        time.Now().Add(time.Hour).Unix(),
	})

	accessToken, err := userAccessToken.SignedString([]byte(os.Getenv("JWTSECRET_INVITE")))

	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"message": "Error creating token",
		})

		return
	}

	ctx.SetCookie(
		"betterDocsAT", // Cookie name
		accessToken,    // Cookie value
		3600*24,        // MaxAge: 1 day in seconds
		"/",            // Path
		"",             // Domain (leave empty for default)
		true,           // Secure (true if using HTTPS)
		true,           // HttpOnly (prevents JavaScript access)
	)

	ctx.JSON(http.StatusOK, gin.H{
		"userDetails": userDetails,
	})

}

func GetUserDetailsFromGithub(token string) (models.Users, error) {
	// Create a new request
	req, err := http.NewRequest("GET", "https://api.github.com/user", nil)
	if err != nil {
		return models.Users{}, fmt.Errorf("error while creating request to GitHub: %w", err)
	}

	// Set the Authorization header with the access token
	req.Header.Set("Authorization", "Bearer "+token)

	// Send the request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return models.Users{}, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return models.Users{}, fmt.Errorf("failed to read response body: %w", err)
	}

	var userDetails map[string]interface{}
	if err := json.Unmarshal(body, &userDetails); err != nil {
		return models.Users{}, fmt.Errorf("error while unmarshalling: %w", err)
	}

	// Check for status code 400
	if resp.StatusCode == 400 {
		if message, exists := userDetails["message"].(string); exists {
			return models.Users{}, fmt.Errorf("bad request: %s", message)
		}
		return models.Users{}, fmt.Errorf("bad request")
	}

	// Extract and convert the GitHub ID
	id, ok := userDetails["id"].(float64)
	if !ok {
		return models.Users{}, errors.New("GitHub ID is not a valid number")
	}
	githubID := int(id)

	// Check if user exists in the database
	var exists bool
	err = initializer.DB.QueryRow(context.Background(), "SELECT EXISTS(SELECT 1 FROM users WHERE github_id = $1)", githubID).Scan(&exists)
	if err != nil {
		return models.Users{}, fmt.Errorf("database query error: %w", err)
	}

	var user models.Users

	// If the user does not exist, insert a new record
	if !exists {
		// Populate user fields from GitHub response
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
		user.GithubID = githubID
		if githubName, ok := userDetails["login"].(string); ok {
			user.GithubName = githubName
		}
		if name, ok := userDetails["name"].(string); ok {
			user.Name = name
		}

		// Insert the new user into the database
		err := initializer.DB.QueryRow(context.Background(),
			`INSERT INTO users (avatar_url, company, email, twitter, github_id, github_name, name)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			RETURNING id, avatar_url, company, email, twitter, github_id, github_name, name`,
			user.AvatarURL, user.Company, user.Email, user.Twitter, user.GithubID, user.GithubName, user.Name).
			Scan(&user.ID, &user.AvatarURL, &user.Company, &user.Email, &user.Twitter, &user.GithubID, &user.GithubName, &user.Name)

		if err != nil {
			return models.Users{}, fmt.Errorf("unable to save data to DB while creating user: %w", err)
		}
	} else {
		// Fetch the existing user from the database
		err = initializer.DB.QueryRow(context.Background(),
			`SELECT id, avatar_url, company, email, twitter, github_id, github_name, name
			 FROM users WHERE github_id = $1`, githubID).
			Scan(&user.ID, &user.AvatarURL, &user.Company, &user.Email, &user.Twitter, &user.GithubID, &user.GithubName, &user.Name)

		if err != nil {
			return models.Users{}, fmt.Errorf("unable to get user: %w", err)
		}
	}

	return user, nil
}

func getAccessToken(code string) (string, error) {
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

	var tokenResponse GitHubTokenResponse
	err = json.Unmarshal(respBody, &tokenResponse)
	if err != nil {
		return "", fmt.Errorf("failed to unmarshal response body: %w", err)
	}

	return tokenResponse.AccessToken, nil
}

type GitHubTokenResponse struct {
	AccessToken string `json:"access_token"`
	Scope       string `json:"scope"`
	TokenType   string `json:"token_type"`
}

func GetUserDetailsFromGithubFromApi(ctx *gin.Context) {
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

func Callback(ctx *gin.Context) {

	provider := ctx.Param("provider")

	// Add provider to the request's context
	newCtx := context.WithValue(ctx.Request.Context(), "provider", provider)
	ctx.Request = ctx.Request.WithContext(newCtx)

	user, err := gothic.CompleteUserAuth(ctx.Writer, ctx.Request)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, err)
		return
	}

	fmt.Println(user)

	ctx.Redirect(http.StatusOK, "http://localhost:5173/login")
}

func Logout(ctx *gin.Context) {
	provider := ctx.Param("provider")
	// Add provider to the request's context
	newCtx := context.WithValue(ctx.Request.Context(), "provider", provider)
	ctx.Request = ctx.Request.WithContext(newCtx)
	gothic.Logout(ctx.Writer, ctx.Request)
	ctx.Redirect(http.StatusTemporaryRedirect, "/")
}

func Auth(ctx *gin.Context) {
	provider := ctx.Param("provider")
	// Add provider to the request's context
	newCtx := context.WithValue(ctx.Request.Context(), "provider", provider)
	ctx.Request = ctx.Request.WithContext(newCtx)
	// try to get the user without re-authenticating
	gothUser, err := gothic.CompleteUserAuth(ctx.Writer, ctx.Request)
	if err != nil {
		gothic.BeginAuthHandler(ctx.Writer, ctx.Request)
	}

	fmt.Println(gothUser)
}
