package api

import (
	"github.com/Akshdhiwar/simpledocs-backend/internals/controller"
	// "github.com/Akshdhiwar/simpledocs-backend/internals/middleware"
	"github.com/gin-gonic/gin"
)

func InviteRoutes(router *gin.RouterGroup) {

	// router.Use(middleware.AuthMiddleware)

	// api to invite a user to project
	router.POST("/create", controller.CreateInvite)

	// api to accept the invite
	router.POST("/accept", controller.AcceptInvite)

}
