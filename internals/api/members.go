package api

import (
	"github.com/Akshdhiwar/simpledocs-backend/internals/controller"
	"github.com/Akshdhiwar/simpledocs-backend/internals/middleware"
	"github.com/gin-gonic/gin"
)

func MemberRoutes(router *gin.RouterGroup) {

	router.Use(middleware.AuthMiddleware, middleware.RoleMiddleware)

	// GET api to list all the github members
	router.GET("/org/:id", controller.GetOrgMembers)

	// GET api to get the details of sinle github user
	router.GET("/:proj/:name", controller.GetUserDetails)

}
