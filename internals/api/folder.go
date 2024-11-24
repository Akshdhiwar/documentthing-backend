package api

import (
	"github.com/Akshdhiwar/simpledocs-backend/internals/controller"
	"github.com/Akshdhiwar/simpledocs-backend/internals/middleware"
	"github.com/gin-gonic/gin"
)

func FolderRoutes(router *gin.RouterGroup) {

	router.Use(middleware.AuthMiddleware)

	// GET Api to get folder of Repo from specified id
	router.GET("/:id/:type", controller.GetFolder)

	// Update Folder
	router.POST("/update", controller.UpdateFolder)

}
