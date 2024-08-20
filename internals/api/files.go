package api

import (
	"github.com/Akshdhiwar/simpledocs-backend/internals/controller"
	"github.com/gin-gonic/gin"
)

func FilesRoutes(router *gin.RouterGroup) {

	// GET api to get file contents
	router.GET("/get", controller.GetFileContents)

	// PUT api to update file contents
	router.PUT("/update", controller.UpdateFileContents)

	// PATCH api to update name of file
	router.POST("/rename", controller.UpdateFileName)

	// Delete api to delete the api routes
	router.DELETE("", controller.DeleteFiles)
}
