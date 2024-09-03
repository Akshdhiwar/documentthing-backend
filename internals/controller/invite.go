package controller

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/Akshdhiwar/simpledocs-backend/internals/initializer"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func CreateInvite(ctx *gin.Context) {
	var body struct {
		GithubName string `json:"github_name"`
		Email      string `json:"email"`
		ProjectID  string `json:"project_id"`
		Role       string `json:"role"`
	}

	err := ctx.ShouldBindJSON(&body)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, "Error while binding json to body")
		return
	}

	if body.GithubName == "" || body.Email == "" || body.ProjectID == "" {
		ctx.JSON(http.StatusBadRequest, "Please provide all the required payload")
		return
	}

	var inviteExists bool

	err = initializer.DB.QueryRow(context.Background(), `
    SELECT
        CASE
            WHEN EXISTS (
                SELECT 1
                FROM invite
                WHERE 
                    user_name = $1
                    AND project_id = $2
                    AND deleted_at IS NULL
                    AND is_accepted IS FALSE
                    AND is_revoked IS FALSE
            ) 
            THEN TRUE
            ELSE FALSE
        END AS invite_exists;
	`, body.GithubName, body.ProjectID).Scan(&inviteExists)

	if err != nil {
		ctx.JSON(http.StatusInternalServerError, "Error while retriving invite data from Database")
		return
	}

	if inviteExists {
		ctx.JSON(http.StatusOK, "User already invited to this project")
		return
	}

	// create a record in invite table
	var id uuid.UUID
	initializer.DB.QueryRow(context.Background(), `
		INSERT INTO invite (email , user_name , project_id , role) VALUES ($1, $2, $3 , $4) RETURNING id
	`, body.Email, body.GithubName, body.ProjectID, body.Role).Scan(&id)

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"githubName": body.GithubName,
		"email":      body.Email,
		"projectID":  body.ProjectID,
		"role":       body.Role,
		"sub":        id,
		"exp":        time.Now().Add(time.Hour * 48).Unix(),
	})

	tokenString, err := token.SignedString([]byte(os.Getenv("JWTSECRET")))

	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"message": "Error creating token",
		})

		return
	}

	ctx.JSON(http.StatusOK, tokenString)

}

func AcceptInvite(ctx *gin.Context) {
	var body struct {
		Name  string `json:"name"`
		Token string `json:"token"`
		ID    string `json:"id"`
	}

	err := ctx.ShouldBindJSON(&body)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, "Error while binding json to body")
		return
	}

	if body.Token == "" || body.Name == "" || body.ID == "" {
		ctx.JSON(http.StatusBadRequest, "Please provide all the details in the API")
		return
	}

	claims, err := parseJWTToken(body.Token, []byte(os.Getenv("JWTSECRET")))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, "Error while parsing JWT token")
		return
	}

	if claims.GithubName != body.Name {
		ctx.JSON(http.StatusForbidden, "Wrong invite")
		return
	}

	var userGithubName string
	err = initializer.DB.QueryRow(context.Background(), `SELECT github_name FROM users WHERE id = $1`, body.ID).Scan(&userGithubName)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, err.Error())
		return
	}

	if claims.GithubName != userGithubName {
		ctx.JSON(http.StatusForbidden, "Wrong invite")
		return
	}

	tx, err := initializer.DB.Begin(context.Background())
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"message": "Unable to create transaction: " + err.Error(),
		})
		return
	}

	// Ensure transaction is committed or rolled back
	defer func() {
		if err != nil {
			tx.Rollback(context.Background())
		} else {
			tx.Commit(context.Background())
		}
	}()

	_, err = tx.Exec(context.Background(), `INSERT INTO user_project_mapping (user_id, project_id , role) VALUES ($1, $2 , $3)`, body.ID, claims.ProjectID, claims.Role)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"message": "Error saving data to DB: " + err.Error(),
		})
		return
	}

	_, err = tx.Exec(context.Background(), `UPDATE invite SET deleted_at = $1 , is_accepted = true WHERE id = $2`, time.Now(), claims.Subject)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"message": "Error saving data to DB: " + err.Error(),
		})
		return
	}

	ctx.JSON(http.StatusOK, "Invite accepted successfully")
}

type Claims struct {
	GithubName string `json:"githubName"`
	Email      string `json:"email"`
	ProjectID  string `json:"projectID"`
	Role       string `json:"role"`
	jwt.RegisteredClaims
}

// This function parses the JWT token and returns all claims
func parseJWTToken(token string, hmacSecret []byte) (claims Claims, err error) {

	// Parse the token and validate the signature
	t, err := jwt.ParseWithClaims(token, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return hmacSecret, nil
	})

	// Check if there's an error in parsing or validating the token
	if err != nil {
		return Claims{}, fmt.Errorf("error parsing or validating token: %v", err)
	}

	// Check if the token is valid and extract all claims
	if claims, ok := t.Claims.(*Claims); ok && t.Valid {
		return *claims, nil
	}

	return Claims{}, fmt.Errorf("invalid token")
}
