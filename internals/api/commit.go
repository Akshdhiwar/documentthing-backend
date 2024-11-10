package api

import (
	"github.com/Akshdhiwar/simpledocs-backend/internals/controller"
	"github.com/Akshdhiwar/simpledocs-backend/internals/middleware"
	"github.com/Akshdhiwar/simpledocs-backend/internals/models"
	"github.com/gin-gonic/gin"
)

func CommitRoutes(router *gin.RouterGroup) {

	router.POST("/save", middleware.AuthMiddleware, middleware.RoleMiddleware([]models.UserRole{models.RoleAdmin, models.RoleEditor}), controller.CommitChanges)

}
