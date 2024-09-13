package api

import (
	"github.com/Akshdhiwar/simpledocs-backend/internals/controller"
	// "github.com/Akshdhiwar/simpledocs-backend/internals/middleware"
	"github.com/gin-gonic/gin"
)

func ProjectRoutes(router *gin.RouterGroup) {

	 // router.Use(middleware.AuthMiddleware)

	// POST route to create new project and repo in github
	router.POST("/create-project", controller.CreateNewProject)

	// GET route to get the user projects
	router.GET("/get-project", controller.GetProjects)
}
