package controller

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/Akshdhiwar/simpledocs-backend/internals/utils"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func CreatePaypalProduct(c *gin.Context) {
	// Check if request body is empty
	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}

	err := c.ShouldBindJSON(&body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Bind JSON body to Product struct
	product := Product{
		Name:        body.Name,
		Description: body.Description,
		Type:        "SERVICE",
		Category:    "SOFTWARE",
	}

	// PayPal API URL for creating a product
	apiURL := "https://api-m.sandbox.paypal.com/v1/catalogs/products"

	// Marshal product to JSON
	productJSON, err := json.Marshal(product)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to marshal product data"})
		return
	}

	// Create a new POST request to PayPal API
	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(productJSON))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create request"})
		return
	}

	// Set headers
	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+utils.PaypalAccessToken)
	req.Header.Set("PayPal-Request-Id", uuid.New().String())

	// Send the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send request"})
		return
	}
	defer resp.Body.Close()

	// Check the response
	if resp.StatusCode == http.StatusCreated {
		c.JSON(http.StatusCreated, gin.H{"message": "Product created successfully"})
	} else {
		c.JSON(resp.StatusCode, gin.H{"error": "Failed to create product", "status": resp.Status})
	}
}

func GetProductsFromPaypal(c *gin.Context) {
	// PayPal API URL for fetching products
	apiURL := "https://api-m.sandbox.paypal.com/v1/catalogs/products"

	// Create a new GET request to PayPal API
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create request"})
		return
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+utils.PaypalAccessToken)
	req.Header.Set("PayPal-Request-Id", uuid.New().String())

	// Send the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send request"})
		return
	}
	defer resp.Body.Close()

	// Check the response
	if resp.StatusCode == http.StatusOK {
		var responseData map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&responseData); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse response"})
			return
		}
		c.JSON(http.StatusOK, responseData)
	} else {
		// Parse error response
		var errorData map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&errorData); err != nil {
			errorData = map[string]interface{}{
				"error": "Failed to parse error response",
			}
		}
		errorData["status"] = resp.Status
		c.JSON(resp.StatusCode, errorData)
	}
}

// Define the payload structure
type Product struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description" binding:"required"`
	Type        string `json:"type" binding:"required"`
	Category    string `json:"category" binding:"required"`
}
