package api

import (
	"github.com/Akshdhiwar/simpledocs-backend/internals/controller"
	"github.com/gin-gonic/gin"
)

func FolderRoutes(router *gin.RouterGroup) {

	// GET Api to get folder of Repo from specified id
	router.GET("/:id", controller.GetFolder)

}
