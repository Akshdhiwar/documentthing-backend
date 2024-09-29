package main

import (
	"os"
	"time"

	"github.com/Akshdhiwar/simpledocs-backend/database"
	"github.com/Akshdhiwar/simpledocs-backend/internals/api"
	"github.com/Akshdhiwar/simpledocs-backend/internals/initializer"
	"github.com/Akshdhiwar/simpledocs-backend/internals/utils"
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

	router.Use(utils.Cors())
	baseRoute := "api/v1"

	defer initializer.DB.Close()

	//default route
	api.Default(router.Group(baseRoute))

	// Handle preflight OPTIONS requests
	router.OPTIONS("/*path", func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "https://simpledocs.vercel.app")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE")
		c.Header("Access-Control-Allow-Headers", "X-User-Id, X-Project-Id, Content-Type, Authorization")
		c.Header("Access-Control-Allow-Credentials", "true")
		c.AbortWithStatus(204)
	})

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

	// api route for commit and save the changes
	api.CommitRoutes(router.Group(baseRoute + "/commit"))

	// Run the server on port 3000
	router.Run()

}
