package utils

import (
	"strings"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func Cors() gin.HandlerFunc {
	config := cors.DefaultConfig()

	config.AllowOrigins = []string{
		"http://localhost:5173",
		"https://simpledocs.vercel.app",
		"https://www.documentthing.com",
		"https://documentthing.com",
		"http://localhost:5174",
	}

	// Dynamically allow subdomains of documentthing.com
	config.AllowOriginFunc = func(origin string) bool {
		return strings.HasSuffix(origin, ".documentthing.com")
	}

	config.AllowMethods = []string{"GET", "POST", "PUT", "DELETE"}
	config.AllowHeaders = []string{"X-User-Id", "X-Project-Id", "Content-Type", "Authorization"}
	config.AllowCredentials = true

	return cors.New(config)
}
