package middleware

import (
	"context"
	"net/http"

	"github.com/Akshdhiwar/simpledocs-backend/internals/initializer"
	"github.com/gin-gonic/gin"
)

func RoleMiddleware(ctx *gin.Context) {

	projectID := ctx.GetHeader("X-Project-Id")
	userID := ctx.GetHeader("X-User-Id")

	if projectID == "" || userID == "" {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"message": "Project or User ID not found",
		})
		return
	}

	var role string

	err := initializer.DB.QueryRow(context.Background(), `SELECT
  			up.role
			FROM
  			user_project_mapping up
			WHERE
  			up.project_id = $1
  			AND up.user_id = $2;`, projectID, userID).Scan(&role)

	if err != nil {
		ctx.JSON(http.StatusInternalServerError, "Error while retriving data from db")
		return
	}

	if role == "Admin" {
		ctx.Next()
	} else {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
			"message": "Unauthorized",
		})
		return
	}

}
