package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/Akshdhiwar/simpledocs-backend/internals/initializer"
	"github.com/Akshdhiwar/simpledocs-backend/internals/models"
	"github.com/gin-gonic/gin"
)

func HandleWebhookEvents(ctx *gin.Context) {
	// Log that the webhook was called
	fmt.Println("Webhook received")

	// Step 1: Parse the webhook event payload
	var event PayPalWebhookEvent
	if err := ctx.ShouldBindJSON(&event); err != nil {
		fmt.Println("Error parsing webhook payload:", err)
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Invalid payload"})
		return
	}

	// Step 2: Handle different event types based on EventType
	switch event.EventType {
	case "BILLING.SUBSCRIPTION.CREATED":
		var resource models.Resource
		if err := json.Unmarshal(event.Resource, &resource); err == nil {
			fmt.Println("Subscription Created:", resource.ID)
			// fmt.Println("Subscription Created:", event)
			// Perform actions such as logging or storing in the database
		}

	case "BILLING.SUBSCRIPTION.ACTIVATED":
		var resource SubscriptionResource
		if err := json.Unmarshal(event.Resource, &resource); err == nil {
			fmt.Println("Subscription Activated:", resource.ID)
			// fmt.Println("Subscription Activated:", event)
			// Update user subscription status in database

			_, err := initializer.DB.Query(context.Background(),
				`UPDATE organizations SET status = true WHERE subscription_id = $1`,
				resource.ID)

			if err != nil {
				ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Error while updating subscription status"})
				return
			}
		}

	case "BILLING.SUBSCRIPTION.CANCELLED":
		var resource SubscriptionResource
		if err := json.Unmarshal(event.Resource, &resource); err == nil {
			fmt.Println("Subscription Cancelled:", resource.ID)
			// Update user subscription status in database
			_, err := initializer.DB.Query(context.Background(), `UPDATE organizations SET status = false WHERE subscription_id = $1 `, resource.ID)

			if err != nil {
				ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Error while updating subscription status"})
				return
			}
		}

	case "BILLING.SUBSCRIPTION.EXPIRED":
		var resource SubscriptionResource
		if err := json.Unmarshal(event.Resource, &resource); err == nil {
			fmt.Println("Subscription Expired:", resource.ID)
			// Update user subscription status in database
			_, err := initializer.DB.Query(context.Background(), `UPDATE organizations SET status = false WHERE subscription_id = $1 `, resource.ID)

			if err != nil {
				ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Error while updating subscription status"})
				return
			}
		}

	// Add other cases as necessary for events like BILLING.SUBSCRIPTION.RENEWED, PAYMENT.SALE.COMPLETED, etc.
	default:
		fmt.Println("Unhandled event type:", event.EventType)
	}

	// Respond with 200 OK to acknowledge receipt of the event
	ctx.JSON(http.StatusOK, gin.H{"message": "Webhook event received"})
}

// Define the general structure of the webhook event
type PayPalWebhookEvent struct {
	EventType string          `json:"event_type"`
	Resource  json.RawMessage `json:"resource"`
}

// Define specific event resources you want to handle (e.g., subscription details)
type SubscriptionResource struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	// Add additional fields if needed
}
