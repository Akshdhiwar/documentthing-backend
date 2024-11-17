package api

import (
	"github.com/Akshdhiwar/simpledocs-backend/internals/controller"
	"github.com/Akshdhiwar/simpledocs-backend/internals/middleware"
	"github.com/Akshdhiwar/simpledocs-backend/internals/utils"
	"github.com/gin-gonic/gin"
)

func ProjectRoutes(router *gin.RouterGroup) {

	router.Use(middleware.AuthMiddleware)

	// POST route to create new project and repo in github
	router.POST("/create-project", controller.CreateNewProject)

	// GET route to get the user projects
	router.GET("/get-project", controller.GetProjects)

	// GET route to get the installation accounts
	router.GET("/installation", controller.GetInstallation)

	// POST route to get access token for github installation
	router.POST("/installation/access_token/:id", controller.GetAccessTokenForGithubAppInstallation)

	// Route for long polling (user waiting for project updates)
	router.GET("/:projectID/updates", utils.HandleWebSocket)
}
