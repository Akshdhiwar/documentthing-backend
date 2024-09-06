package api

import (
	"github.com/Akshdhiwar/simpledocs-backend/internals/controller"
	"github.com/gin-gonic/gin"
)

func AccountRoutes(router *gin.RouterGroup) {

	// POST Api to get access token from github for the user
	router.POST("/get-access-token", controller.GetAccessTokenFromGithub)

	// GET Api to get the user details which requires a github access token in Authorization headers
	router.GET("/user-details", controller.GetUserDetailsFromGithubFromApi)

	// GET Api for callback
	router.GET("/auth/:provider/callback", controller.Callback)

	// GET Api for callback
	router.GET("/logout/:provider", controller.Callback)

	// GET Api for callback
	router.GET("/auth/:provider", controller.Auth)

}
