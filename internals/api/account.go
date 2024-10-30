package api

import (
	"github.com/Akshdhiwar/simpledocs-backend/internals/controller"
	"github.com/Akshdhiwar/simpledocs-backend/internals/middleware"
	"github.com/gin-gonic/gin"
)

func AccountRoutes(router *gin.RouterGroup) {

	// POST Api to get access token from github for the user
	router.POST("/get-access-token", controller.GetAccessTokenFromGithub)

	// GET Api to get the user details which requires a github access token in Authorization headers
	router.GET("/user-details", middleware.AuthMiddleware, controller.GetUserDetailsFromGithubFromApi)

	// Get api to get user organization
	router.GET("/org", middleware.AuthMiddleware, controller.GetUserOrganization)

	// POST api to create a 6 digit otp to verify and add email
	router.POST("/add-email", middleware.AuthMiddleware, controller.CreateEmailOtp)

	// POST api to verify the otp
	router.POST("/verify-otp", middleware.AuthMiddleware, controller.VerifyOtp)

}
