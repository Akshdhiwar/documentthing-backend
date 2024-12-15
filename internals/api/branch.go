package api

import (
	"github.com/Akshdhiwar/simpledocs-backend/internals/controller"
	"github.com/Akshdhiwar/simpledocs-backend/internals/middleware"
	"github.com/Akshdhiwar/simpledocs-backend/internals/models"
	"github.com/gin-gonic/gin"
)

func BranchRoutes(router *gin.RouterGroup) {

	router.Use(middleware.AuthMiddleware, middleware.RoleMiddleware([]models.UserRole{models.RoleEditor, models.RoleAdmin}))

	router.POST("", controller.CreateBranchForEditing)

	router.DELETE("/:project_id/:branch_name", controller.DeleteBranch)

	router.GET("/check/:id", controller.CheckIfEditingBranchExists)

}
