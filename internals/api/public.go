package api

import (
	"github.com/Akshdhiwar/simpledocs-backend/internals/controller"
	"github.com/Akshdhiwar/simpledocs-backend/internals/middleware"
	"github.com/Akshdhiwar/simpledocs-backend/internals/models"
	"github.com/gin-gonic/gin"
)

func PublicRoutes(router *gin.RouterGroup) {
	router.GET("/:name/folder", controller.GetPublicFolder)

	router.GET("/:name/file/:id", controller.GetPublicFile)

	router.POST("/publish", middleware.AuthMiddleware, middleware.RoleMiddleware([]models.UserRole{models.RoleAdmin}), controller.PublishDocs)
}
