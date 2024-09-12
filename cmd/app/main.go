package main

import (
	"os"
	"time"

	"github.com/Akshdhiwar/simpledocs-backend/database"
	"github.com/Akshdhiwar/simpledocs-backend/internals/api"
	"github.com/Akshdhiwar/simpledocs-backend/internals/initializer"
	"github.com/Akshdhiwar/simpledocs-backend/internals/utils"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/go-co-op/gocron"
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
	// Create a new scheduler in the local time zone
	scheduler := gocron.NewScheduler(time.Local)

	// You can also set it to run every hour, or any other interval
	scheduler.Every(1).Hour().Do(utils.DeleteExpiredInvites)

	// Start the scheduler in blocking mode
	scheduler.StartAsync()

	// router.Use(utils.Cors())
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:5173", "https://simpledocs.vercel.app"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE"},
		AllowHeaders:     []string{"Authorization", "Content-Type", "X-User-Id", "X-Project-Id"},
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

	// api routes for organization
	api.OrgRoutes(router.Group(baseRoute + "/orgs"))

	// api routes for members
	api.MemberRoutes(router.Group(baseRoute + "/member"))

	// api route for invites
	api.InviteRoutes(router.Group(baseRoute + "/invite"))

	// Run the server on port 3000
	router.Run()

}
