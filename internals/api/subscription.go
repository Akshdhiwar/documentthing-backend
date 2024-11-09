package api

import (
	"github.com/Akshdhiwar/simpledocs-backend/internals/controller"
	"github.com/Akshdhiwar/simpledocs-backend/internals/middleware"
	"github.com/gin-gonic/gin"
)

func SubscriptionRoutes(router *gin.RouterGroup) {

	router.POST("/plan/create", middleware.AuthenticateAdminApi, controller.CreatePaypalSubscriptionPlan)

	router.GET("/plan/list", middleware.AuthenticateAdminApi, controller.GetPaypalSubscriptionPlans)

	router.POST("", controller.AddSubcription)

	router.GET("/details/:id", controller.GetSubscriptionDetails)

	router.GET("/plan/details/:id", controller.GetPayPalPlanDetailsHandler)

	router.POST("/delete", controller.DeleteSubscriptionPlan)

	router.POST("/plan/update", controller.UpdatePaypalPlanPricing)

}
