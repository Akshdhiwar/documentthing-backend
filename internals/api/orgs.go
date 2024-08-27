package api

import (
	"github.com/Akshdhiwar/simpledocs-backend/internals/controller"
	"github.com/gin-gonic/gin"
)

func OrgRoutes(router *gin.RouterGroup) {

	// GET Api to get orgs for the user
	router.GET("", controller.GetOrganization)

}
