package utils

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type Update struct { // The updated content
	UpdatedBy string `json:"updatedBy"` // The userID of the user who made the update
}

type Project struct {
	Users []chan Update // Channels for each user working on the project
}

var (
	projects = make(map[string]*Project) // Map of project IDs to Project structure
	mutex    = &sync.Mutex{}             // Protect access to the map
)

// LongPollHandler handles the long polling requests from users working on a project
func LongPollHandler(c *gin.Context) {
	projectID := c.Param("projectID")

	// Add the user to the project's user list
	userChan := addUserToProject(projectID)

	// Ensure that the user is removed from the project list when done
	defer removeUserFromProject(projectID, userChan)

	select {
	case update := <-userChan:
		// Return the update along with the user who made the update
		c.JSON(http.StatusOK, gin.H{
			"updatedBy": update.UpdatedBy,
		})
	case <-time.After(300 * time.Second): // Timeout after 300 seconds
		c.JSON(http.StatusNoContent, gin.H{"message": "No updates"})
	}
}

// Function to add a user to a project, creating a new channel for that user
func addUserToProject(projectID string) chan Update {
	mutex.Lock()
	defer mutex.Unlock()

	if _, exists := projects[projectID]; !exists {
		projects[projectID] = &Project{Users: []chan Update{}}
	}

	// Create a new channel for the user
	userChannel := make(chan Update)
	projects[projectID].Users = append(projects[projectID].Users, userChannel)
	return userChannel
}

// NotifyUsers notifies all users in the project about an update, including who made the update
func NotifyUsers(projectID string, updatedBy string) {
	mutex.Lock()
	defer mutex.Unlock()

	if project, exists := projects[projectID]; exists {
		update := Update{
			UpdatedBy: updatedBy, // The user who made the update
		}

		for _, userChan := range project.Users {
			// Non-blocking send (if the user is still connected)
			select {
			case userChan <- update:
			default:
			}
		}
	}
}

// Function to remove a user from a project
func removeUserFromProject(projectID string, userChan chan Update) {
	mutex.Lock()
	defer mutex.Unlock()

	if project, exists := projects[projectID]; exists {
		for i, ch := range project.Users {
			if ch == userChan {
				projects[projectID].Users = append(project.Users[:i], project.Users[i+1:]...)
				close(ch) // Close the channel to signal disconnection
				break
			}
		}
	}
}
