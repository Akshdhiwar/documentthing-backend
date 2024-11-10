package api

import (
	"github.com/Akshdhiwar/simpledocs-backend/internals/controller"
	"github.com/Akshdhiwar/simpledocs-backend/internals/middleware"
	"github.com/gin-gonic/gin"
)

func SubscriptionRoutes(router *gin.RouterGroup) {

	router.POST("/plan/create", middleware.AuthenticateSuperAdminApi, controller.CreatePaypalSubscriptionPlan)

	router.GET("/plan/list", middleware.AuthenticateSuperAdminApi, controller.GetPaypalSubscriptionPlans)

	router.GET("/plan/details/:id", middleware.AuthenticateSuperAdminApi, controller.GetPayPalPlanDetailsHandler)

	router.POST("/delete", middleware.AuthenticateSuperAdminApi, controller.DeleteSubscriptionPlan)

	router.POST("/plan/update", middleware.AuthenticateSuperAdminApi, controller.UpdatePaypalPlanPricing)

	// API for users subscription

	router.POST("", controller.AddSubcription)

	router.GET("/details/:id", controller.GetSubscriptionDetails)
}
