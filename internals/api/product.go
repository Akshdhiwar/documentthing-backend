package api

import (
	"github.com/Akshdhiwar/simpledocs-backend/internals/controller"
	"github.com/Akshdhiwar/simpledocs-backend/internals/middleware"
	"github.com/gin-gonic/gin"
)

func ProductRoutes(router *gin.RouterGroup) {

	router.POST("create", middleware.AuthenticateSuperAdminApi, controller.CreatePaypalProduct)

	router.GET("list", middleware.AuthenticateSuperAdminApi, controller.GetProductsFromPaypal)

}
