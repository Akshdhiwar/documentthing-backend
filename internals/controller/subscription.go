package controller

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Akshdhiwar/simpledocs-backend/internals/initializer"
	"github.com/Akshdhiwar/simpledocs-backend/internals/models"
	"github.com/Akshdhiwar/simpledocs-backend/internals/utils"
	"github.com/gin-gonic/gin"
)

func AddSubcription(ctx *gin.Context) {
	var body struct {
		SubID        string `json:"sub_id"`
		OrgID        string `json:"org_id"`
		MaxUserCount int8   `json:"max_user_count"`
		SubName      string `json:"sub_name"`
		PlanID       string `json:"plan_id"`
	}

	err := ctx.ShouldBindJSON(&body)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, "Error while binding body")
		return
	}

	_, err = initializer.DB.Query(
		context.Background(),
		`UPDATE organizations SET subscription_id = $1 , subs_name = $2 , max_user = $3 , plan_id = $4 WHERE id = $5`,
		body.SubID,
		body.SubName,
		body.MaxUserCount,
		body.PlanID,
		body.OrgID,
	)

	if err != nil {
		ctx.JSON(http.StatusInternalServerError, "Error while updating subscription")
		return
	}

	ctx.JSON(http.StatusOK, "Subscription updated successfully")

}

func GetSubscriptionDetails(ctx *gin.Context) {
	orgID := ctx.Param("id")

	var subID string
	err := initializer.DB.QueryRow(
		context.Background(),
		`SELECT subscription_id FROM organizations WHERE id = $1`,
		orgID,
	).Scan(&subID)

	if err != nil {
		if err == sql.ErrNoRows {
			// If no subscription ID is found, return a 404 error
			ctx.JSON(http.StatusNotFound, gin.H{"error": "Subscription not found for organization"})
		} else {
			// Other database errors
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Error while retrieving subscription"})
		}
		return
	}

	// Make request to PayPal subscription API and get the subscription details
	subscription, err := getSubscriptionDetailsFromPaypal(subID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Error fetching subscription details from PayPal"})
		fmt.Printf("Error fetching subscription details: %v\n", err)
		return
	}

	ctx.JSON(http.StatusOK, subscription)
}

// getSubscriptionDetailsFromPaypal fetches subscription details from PayPal
func getSubscriptionDetailsFromPaypal(id string) (*models.Subscription, error) {

	url := fmt.Sprintf("https://api-m.sandbox.paypal.com/v1/billing/subscriptions/%s", id)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+utils.PaypalAccessToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to get subscription details: %v - %s", resp.Status, body)
	}

	var subscription models.Subscription
	if err := json.NewDecoder(resp.Body).Decode(&subscription); err != nil {
		return nil, err
	}

	return &subscription, nil
}

// Function to create PayPal subscription plan
func CreatePaypalSubscriptionPlan(ctx *gin.Context) {
	var request SubscriptionPlanRequest
	if err := ctx.ShouldBindJSON(&request); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	url := "https://api-m.sandbox.paypal.com/v1/billing/plans"

	plan := map[string]interface{}{
		"product_id":  request.ProductID,
		"name":        request.Name,
		"description": request.Description,
		"billing_cycles": []map[string]interface{}{
			{
				"frequency": map[string]interface{}{
					"interval_unit":  "MONTH",
					"interval_count": "1",
				},
				"tenure_type":  "REGULAR",
				"sequence":     1,
				"total_cycles": 0, // 0 for infinite cycles
				"pricing_scheme": map[string]interface{}{
					"fixed_price": map[string]interface{}{
						"value":         request.Price,
						"currency_code": "USD",
					},
				},
			},
		},
		"quantity_supported": true,
		"payment_preferences": map[string]interface{}{
			"auto_bill_outstanding": true,
			"setup_fee": map[string]string{
				"value":         "0",
				"currency_code": request.Currency,
			},
			"setup_fee_failure_action":  "CONTINUE",
			"payment_failure_threshold": 3,
		},
	}

	payload, _ := json.Marshal(plan)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create request"})
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+utils.PaypalAccessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to make PayPal API request"})
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated {
		ctx.JSON(resp.StatusCode, gin.H{"error": "failed to create subscription plan", "response": string(body)})
		return
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse response"})
		return
	}

	ctx.JSON(http.StatusCreated, gin.H{"message": "subscription plan created", "plan": result})
}

// Struct to parse subscription plan request
type SubscriptionPlanRequest struct {
	ProductID   string `json:"product_id" binding:"required"`
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	Price       string `json:"price" binding:"required"`
	Currency    string `json:"currency" binding:"required"`
}

// Function to get PayPal subscription plans
func GetPaypalSubscriptionPlans(ctx *gin.Context) {

	url := "https://api-m.sandbox.paypal.com/v1/billing/plans?page_size=20" // Customize query params if needed

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create request"})
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+utils.PaypalAccessToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to make PayPal API request"})
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		ctx.JSON(resp.StatusCode, gin.H{"error": "failed to retrieve subscription plans", "response": string(body)})
		return
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse response"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"plans": result})
}

func DeleteSubscriptionPlan(ctx *gin.Context) {
	var body struct {
		PlanID string `json:"plan_id"`
	}

	// Bind JSON body to retrieve Plan ID
	err := ctx.ShouldBindJSON(&body)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Error while binding body"})
		return
	}

	if body.PlanID == "" {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "Plan ID is required"})
		return
	}

	// PayPal API endpoint for deactivating a plan
	url := fmt.Sprintf("https://api-m.sandbox.paypal.com/v1/billing/plans/%s/deactivate", body.PlanID)

	// Create a new POST request for deactivation
	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create request"})
		return
	}

	// Set authorization and content headers
	req.Header.Set("Authorization", "Bearer "+utils.PaypalAccessToken)
	req.Header.Set("Content-Type", "application/json")

	// Execute the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send request to PayPal"})
		return
	}
	defer resp.Body.Close()

	// Check response status for success
	if resp.StatusCode == http.StatusNoContent {
		ctx.JSON(http.StatusOK, gin.H{"message": "Subscription plan deactivated successfully"})
	} else {
		ctx.JSON(resp.StatusCode, gin.H{"error": "Failed to deactivate subscription plan"})
	}
}
