package utils

import (
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func Cors() gin.HandlerFunc {
	config := cors.DefaultConfig()
	config.AllowOrigins = []string{"http://localhost:5173", "https://simpledocs.vercel.app", "https://www.documentthing.com"} // List specific origins
	config.AllowMethods = []string{"GET", "POST", "PUT", "DELETE"}                                                        // Allow specific methods
	config.AllowHeaders = []string{"X-User-Id", "X-Project-Id", "Content-Type", "Authorization"}                          // Include custom headers
	config.AllowCredentials = true                                                                                        // Allow credentials (cookies/authorization headers)
	return cors.New(config)
}
