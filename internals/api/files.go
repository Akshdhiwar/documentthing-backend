package api

import (
	"github.com/Akshdhiwar/simpledocs-backend/internals/controller"
	"github.com/Akshdhiwar/simpledocs-backend/internals/middleware"
	"github.com/gin-gonic/gin"
)

func FilesRoutes(router *gin.RouterGroup) {

	router.Use(middleware.AuthMiddleware)

	// GET api to get file contents
	router.GET("/get", controller.GetFileContents)

	// GET api to get drawings
	router.GET("/drawings", controller.GetDrawings)

	// Get api to get the drawings of specific drawing
	router.GET("/drawings/:name", controller.GetSpecificDrawing)

	// PUT api to update file contents
	router.PUT("/update", controller.UpdateFileContents)

	// PATCH api to update name of file
	router.POST("/rename", controller.UpdateFileName)

	// Delete api to delete the api routes
	router.DELETE("", controller.DeleteFiles)
}
