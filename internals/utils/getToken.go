package utils

import (
	"context"
	"os"

	"github.com/Akshdhiwar/simpledocs-backend/internals/initializer"
	"github.com/gin-gonic/gin"
)

func GetAccessTokenFromBackend(ctx *gin.Context) (string, error) {

	id := ctx.GetHeader("X-User-Id")

	var encryptedToken, name string
	var githubID int

	err := initializer.DB.QueryRow(context.Background(), `SELECT github_token , github_name , github_id from users WHERE id = $1`, id).Scan(&encryptedToken, &name, &githubID)
	if err != nil {
		return "", err
	}

	key := DeriveKey(name + os.Getenv("ENC_SECRET") + string(githubID))

	token, err := Decrypt(encryptedToken, key)
	if err != nil {
		return "", err
	}

	return token, nil
}
