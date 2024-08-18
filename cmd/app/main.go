package main

import (
	"os"

	"github.com/Akshdhiwar/simpledocs-backend/database"
	"github.com/Akshdhiwar/simpledocs-backend/internals/api"
	"github.com/Akshdhiwar/simpledocs-backend/internals/initializer"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func init() {
	if _, exists := os.LookupEnv("RAILWAY_ENVIRONMENT"); !exists {
		initializer.LoadEnvVariables()
	}
	initializer.ConnectToDB()
	database.Migrations()
}

func main() {
	// Create a new Gin router
	router := gin.Default()

	// router.Use(utils.Cors())

	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:5173"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE"},
		AllowHeaders:     []string{"Authorization", "Content-Type"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	baseRoute := "api/v1"

	defer initializer.DB.Close()

	//default route
	api.Default(router.Group(baseRoute))

	// api route for Signup and Login
	api.AccountRoutes(router.Group(baseRoute + "/account"))

	//api route for project
	api.ProjectRoutes(router.Group(baseRoute + "/project"))

	//api routes for folder
	api.FolderRoutes(router.Group(baseRoute + "/folder"))

	//api routes for files
	api.FilesRoutes(router.Group(baseRoute + "/file"))

	// Run the server on port 3000
	router.Run()

}
