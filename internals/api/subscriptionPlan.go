package api

import (
	"github.com/Akshdhiwar/simpledocs-backend/internals/controller"
	"github.com/Akshdhiwar/simpledocs-backend/internals/middleware"
	"github.com/gin-gonic/gin"
)

func SubscriptionPlanRoutes(router *gin.RouterGroup) {

	router.POST("create", middleware.AuthenticateAdminApi, controller.CreatePaypalSubscriptionPlan)

	router.GET("list", middleware.AuthenticateAdminApi, controller.GetPaypalSubscriptionPlans)

}
