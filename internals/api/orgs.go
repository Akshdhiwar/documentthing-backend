package api

import (
	"github.com/Akshdhiwar/simpledocs-backend/internals/controller"
	"github.com/Akshdhiwar/simpledocs-backend/internals/middleware"
	"github.com/gin-gonic/gin"
)

func OrgRoutes(router *gin.RouterGroup) {

	router.Use(middleware.AuthMiddleware)

	// GET Api to get orgs for the user
	router.GET("", controller.GetOrganization)

	router.GET("/:id/members", controller.GetOrgMembersAdminOnly)

}
