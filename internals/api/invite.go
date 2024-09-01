package api

import (
	"github.com/Akshdhiwar/simpledocs-backend/internals/controller"
	"github.com/gin-gonic/gin"
)

func InviteRoutes(router *gin.RouterGroup) {

	// api to invite a user to project
	router.POST("/create", controller.CreateInvite)

	// api to accept the invite
	router.POST("/accept", controller.AcceptInvite)

}
