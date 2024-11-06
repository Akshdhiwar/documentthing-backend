package controller

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/Akshdhiwar/simpledocs-backend/internals/initializer"
	"github.com/Akshdhiwar/simpledocs-backend/internals/models"
	"github.com/Akshdhiwar/simpledocs-backend/internals/utils"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
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
	fmt.Println(token)
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
		"exp":        time.Now().Add(time.Second * 24).Unix(),
	})

	accessToken, err := userAccessToken.SignedString([]byte(os.Getenv("JWTSECRET_ACCESS")))

	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"message": "Error creating token",
		})

		return
	}

	userRefreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"githubName": userDetails.GithubName,
		"email":      userDetails.Email,
		"sub":        userDetails.ID,
		"exp":        time.Now().Add(time.Hour * 24 * 30).Unix(), // 30 days
	})

	refreshToken, err := userRefreshToken.SignedString([]byte(os.Getenv("JWTSECRET_REFRESH")))

	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"message": "Error creating refresh token",
		})

		return
	}

	ctx.SetSameSite(http.SameSiteNoneMode)

	ctx.SetCookie(
		"betterDocsAT", // Cookie name
		accessToken,    // Cookie value
		3600*24,        // MaxAge: 1 day in seconds
		"/",            // Path
		"",             // Domain (leave empty for default)
		true,           // Secure (true if using HTTPS)
		true,           // HttpOnly (prevents JavaScript access)
	)

	ctx.SetCookie(
		"betterDocsRT", // Cookie name
		refreshToken,   // Cookie value
		86400*30,       // MaxAge: 1 day in seconds
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

	// Handle response from GitHub API
	if resp.StatusCode != http.StatusOK {
		return models.Users{}, fmt.Errorf("failed to get user details: %s", resp.Status)
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return models.Users{}, fmt.Errorf("failed to read response body: %w", err)
	}

	var userDetails GitHubUser
	if err := json.Unmarshal(body, &userDetails); err != nil {
		return models.Users{}, fmt.Errorf("error while unmarshalling: %w", err)
	}

	// Check for status code 400

	fmt.Println(resp.StatusCode, token)
	if resp.StatusCode != http.StatusOK {
		return models.Users{}, fmt.Errorf("bad request")
	}

	githubID := userDetails.ID

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
		user.AvatarURL = userDetails.AvatarURL
		user.Company = userDetails.Company
		user.Email = userDetails.Email
		user.Twitter = userDetails.TwitterUsername
		user.GithubID = userDetails.ID
		user.GithubName = userDetails.Login
		user.Name = userDetails.Name

		// Start a new transaction to insert the new user into the database
		tx, err := initializer.DB.Begin(context.Background())
		if err != nil {
			return models.Users{}, fmt.Errorf("unable to start a transaction: %w", err)
		}

		// Ensure transaction is committed or rolled back
		defer func() {
			if p := recover(); p != nil {
				tx.Rollback(context.Background()) // Rollback in case of a panic
				panic(p)
			} else if err != nil {
				tx.Rollback(context.Background()) // Rollback if error occurs
			} else {
				err = tx.Commit(context.Background()) // Commit if no error
			}
		}()

		// Insert the new user into the users table
		err = tx.QueryRow(context.Background(),
			`INSERT INTO users (avatar_url, company, email, twitter, github_id, github_name, name)
	 VALUES ($1, $2, $3, $4, $5, $6, $7)
	 RETURNING id, avatar_url, company, email, twitter, github_id, github_name, name`,
			user.AvatarURL, user.Company, user.Email, user.Twitter, user.GithubID, user.GithubName, user.Name).
			Scan(&user.ID, &user.AvatarURL, &user.Company, &user.Email, &user.Twitter, &user.GithubID, &user.GithubName, &user.Name)

		if err != nil {
			return models.Users{}, fmt.Errorf("unable to save user to DB: %w", err)
		}

		// Insert a new entry into the organization table and return the uuid
		var organizationUUID string
		err = tx.QueryRow(context.Background(),
			`INSERT INTO organizations (owner_id, name)
	 VALUES ($1, $2)
	 RETURNING id`,
			user.ID, user.GithubName).Scan(&organizationUUID)

		if err != nil {
			return models.Users{}, fmt.Errorf("unable to insert into organization: %w", err)
		}

		_, err = tx.Exec(context.Background(), `INSERT INTO org_user_mapping (org_id, user_id) VALUES ($1, $2 )`, organizationUUID, user.ID)
		if err != nil {
			return models.Users{}, fmt.Errorf("unable to insert into organization: %w", err)
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
	var clientID, clientSecret string
	clientID = os.Getenv("GITHUB_APP_CLIENT")
	clientSecret = os.Getenv("GITHUB_APP_CLIENT_SECRET")

	if clientID == "" || clientSecret == "" {
		return "", fmt.Errorf("missing client ID or secret")
	}

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

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Read and parse the response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	var tokenResponse GitHubTokenResponse
	err = json.Unmarshal(respBody, &tokenResponse)
	if err != nil {
		return "", fmt.Errorf("failed to unmarshal response body: %w", err)
	}

	if tokenResponse.AccessToken == "" {
		return "", fmt.Errorf("access token not found in response")
	}

	// Log the token response for debugging (use a logging framework in production)
	fmt.Printf("Token Response: %+v\n", tokenResponse)

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

	token, err := utils.GetAccessTokenFromBackend(ctx)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, "Error while getting details from DB")
		return
	}

	// Set the Authorization header with the token from the request header
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	// Send the request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, "Request failed:"+err.Error())
		return
	}
	defer resp.Body.Close() // Ensure the response body is closed

	if resp.StatusCode != http.StatusOK {
		ctx.JSON(http.StatusInternalServerError, fmt.Sprintf("Unexpected status code: %d", resp.StatusCode))
		return
	}

	// Read the response body
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, "Failed to read response body:"+err.Error())
	}

	var userDetails GitHubUser

	err = json.Unmarshal(body, &userDetails)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, "Error while Unmarshalling")
		return
	}

	id := userDetails.ID

	var exists bool
	err = initializer.DB.QueryRow(context.Background(), `SELECT EXISTS 
	(
    SELECT
      1
    FROM
      users
    WHERE
      github_id = $1
  ) AS EXISTS;
`, id).Scan(&exists)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"message": err.Error(),
			"type":    "error",
		})
		return
	}

	var user models.Users

	if !exists {

		user.AvatarURL = userDetails.AvatarURL
		user.Company = userDetails.Company
		user.Email = userDetails.Email
		user.Twitter = userDetails.TwitterUsername
		user.GithubID = userDetails.ID
		user.GithubName = userDetails.Login
		user.Name = userDetails.Name

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

// GitHubUser represents a GitHub user with nil values handled as empty strings
type GitHubUser struct {
	AvatarURL               string     `json:"avatar_url"`
	Bio                     string     `json:"bio"`
	Blog                    string     `json:"blog"`
	Collaborators           int        `json:"collaborators"`
	Company                 string     `json:"company"`
	CreatedAt               string     `json:"created_at"`
	DiskUsage               int        `json:"disk_usage"`
	Email                   string     `json:"email"`
	EventsURL               string     `json:"events_url"`
	Followers               int        `json:"followers"`
	FollowersURL            string     `json:"followers_url"`
	Following               int        `json:"following"`
	FollowingURL            string     `json:"following_url"`
	GistsURL                string     `json:"gists_url"`
	GravatarID              string     `json:"gravatar_id"`
	Hireable                string     `json:"hireable"`
	HTMLURL                 string     `json:"html_url"`
	ID                      int64      `json:"id"`
	Location                string     `json:"location"`
	Login                   string     `json:"login"`
	Name                    string     `json:"name"`
	NodeID                  string     `json:"node_id"`
	NotificationEmail       string     `json:"notification_email"`
	OrganizationsURL        string     `json:"organizations_url"`
	OwnedPrivateRepos       int        `json:"owned_private_repos"`
	Plan                    GitHubPlan `json:"plan"`
	PrivateGists            int        `json:"private_gists"`
	PublicGists             int        `json:"public_gists"`
	PublicRepos             int        `json:"public_repos"`
	ReceivedEventsURL       string     `json:"received_events_url"`
	ReposURL                string     `json:"repos_url"`
	SiteAdmin               bool       `json:"site_admin"`
	StarredURL              string     `json:"starred_url"`
	SubscriptionsURL        string     `json:"subscriptions_url"`
	TotalPrivateRepos       int        `json:"total_private_repos"`
	TwitterUsername         string     `json:"twitter_username"`
	TwoFactorAuthentication bool       `json:"two_factor_authentication"`
	Type                    string     `json:"type"`
	UpdatedAt               string     `json:"updated_at"`
	URL                     string     `json:"url"`
}

// GitHubPlan represents the plan details within a GitHub user response
type GitHubPlan struct {
	Collaborators int    `json:"collaborators"`
	Name          string `json:"name"`
	PrivateRepos  int    `json:"private_repos"`
	Space         int64  `json:"space"`
}

func GetUserOrganization(ctx *gin.Context) {

	// get user id from headers
	userID := ctx.GetHeader("X-User-Id")

	// check for empty user id
	if userID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"message": "User ID is required",
			"type":    "error",
		})
		return
	}

	// get the organization id from database
	org, err := getOrganizationFromDB(userID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"message": "Error while getting organization from DB",
			"type":    "error",
		})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"org": org,
	})

}

// func for getOrganizationIDFromDB

func getOrganizationFromDB(userID string) (Organization, error) {
	var org Organization
	err := initializer.DB.QueryRow(context.Background(), `SELECT id, name FROM organizations WHERE owner_id = $1`, userID).Scan(&org.ID, &org.Name)
	if err == sql.ErrNoRows {
		return org, fmt.Errorf("no organization found for user with ID %s", userID)
	}
	return org, err
}

type Organization struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

// Struct to hold OTP details
type OtpDetails struct {
	OTP        string
	Email      string
	ValidUntil time.Time // Add a field to manage OTP expiration
}

// Global variable to store OTPs
var otpStore = struct {
	sync.RWMutex
	data map[string]OtpDetails
}{data: make(map[string]OtpDetails)}

func CreateEmailOtp(ctx *gin.Context) {
	var body struct {
		Email string `json:"email"`
	}

	userID := ctx.GetHeader("X-User-Id")

	err := ctx.ShouldBindJSON(&body)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// generate OTP
	otp := generateOTP()

	// Save OTP details in the hashmap
	otpStore.Lock() // Lock the map for writing
	otpStore.data[userID] = OtpDetails{
		OTP:        otp,
		Email:      body.Email,
		ValidUntil: time.Now().Add(5 * time.Minute), // Example expiration time
	}
	otpStore.Unlock() // Unlock the map
	// send OTP to email
	err = sendEmail(body.Email, otp)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Error while sending OTP"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "OTP sent successfully", otp: otp})
}

func generateOTP() string {
	// Create a new random number generator
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	otp := r.Intn(1000000)          // Generates a random number between 0 and 999999
	return fmt.Sprintf("%06d", otp) // Formats as a 6-digit string with leading zeros if needed
}

func sendEmail(email string, otp string) error {
	// use your email service provider here
	// ...
	fmt.Println(email, otp)
	return nil
}

func VerifyOtp(ctx *gin.Context) {
	var body struct {
		OTP string `json:"otp"`
	}

	userID := ctx.GetHeader("X-User-Id")

	err := ctx.ShouldBindJSON(&body)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	otpStore.Lock() // Lock the map for reading
	details, ok := otpStore.data[userID]
	otpStore.Unlock() // Unlock the map

	if !ok || details.ValidUntil.Before(time.Now()) {
		ctx.JSON(http.StatusForbidden, gin.H{"error": "Invalid OTP or OTP expired"})
		return
	}

	if details.OTP != body.OTP {
		ctx.JSON(http.StatusForbidden, gin.H{"error": "Invalid OTP"})
		return
	}

	// Update user email in the database
	var userUUID uuid.UUID

	err = initializer.DB.QueryRow(context.Background(), `
    UPDATE users
    SET email = $1
    WHERE id = $2
    RETURNING id;`, details.Email, userID).Scan(&userUUID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Error while saving user to DB"})
		return
	}

	_, err = initializer.DB.Query(context.Background(), `UPDATE organizations
	SET email = $1
	WHERE owner_id = $2;`, details.Email, userUUID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Error while saving user to DB"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "OTP verified successfully"})
	delete(otpStore.data, userID) // Delete OTP details from the map after successful verification
}

func GetAccountStatus(ctx *gin.Context) {
	// get user id from headers
	userID := ctx.GetHeader("X-User-Id")

	// check for empty user id
	if userID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"message": "User ID is required",
			"type":    "error",
		})
		return
	}

	var status bool

	err := initializer.DB.QueryRow(context.Background(), `SELECT status FROM organizations WHERE owner_id = $1`, userID).Scan(&status)
	if err == sql.ErrNoRows {
		ctx.JSON(http.StatusNotFound, gin.H{
			"message": "No organization found for user with ID " + userID,
			"type":    "error",
		})
		return
	} else if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"message": "Error while retrieving account status",
			"type":    "error",
		})
		return
	}
	ctx.JSON(http.StatusOK, status)
}
