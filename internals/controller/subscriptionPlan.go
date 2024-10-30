package controller

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/Akshdhiwar/simpledocs-backend/internals/utils"
	"github.com/gin-gonic/gin"
)

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
				"frequency": map[string]string{
					"interval_unit":  "MONTH",
					"interval_count": "1",
				},
				"tenure_type":  "REGULAR",
				"sequence":     1,
				"total_cycles": 0, // 0 for infinite cycles
				"pricing_scheme": map[string]interface{}{
					"fixed_price": map[string]string{
						"value":         request.Price,
						"currency_code": request.Currency,
					},
				},
			},
		},
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

	url := "https://api-m.sandbox.paypal.com/v1/billing/plans?page_size=10&page=1&total_required=true" // Customize query params if needed

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
