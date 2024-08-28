package api

import (
	"github.com/Akshdhiwar/simpledocs-backend/internals/controller"
	"github.com/gin-gonic/gin"
)

func MemberRoutes(router *gin.RouterGroup) {

	// GET api to list all the github members
	router.GET("/:id", controller.GetOrgMembers)

}
