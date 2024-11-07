package controller

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Akshdhiwar/simpledocs-backend/internals/initializer"
	"github.com/Akshdhiwar/simpledocs-backend/internals/models"
	"github.com/Akshdhiwar/simpledocs-backend/internals/utils"
	"github.com/gin-gonic/gin"
)

func GetOrganization(ctx *gin.Context) {
	orgs, err := getOrgs(ctx)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, err.Error())
		return
	}

	ctx.JSON(http.StatusOK, orgs)
}

func getOrgs(ctx *gin.Context) ([]models.Organization, error) {
	// Create a new HTTP request to GitHub API
	url := "https://api.github.com/user/orgs" // Note: Use "user/orgs" for authenticated user's orgs
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create new HTTP request: %w", err)
	}
	token, err := utils.GetAccessTokenFromBackend(ctx)
	if err != nil {
		return nil, err
	}

	// Set the Authorization header with the token from the request header
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("Content-Type", "application/json")

	// Make the HTTP request to GitHub API
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make HTTP request: %w", err)
	}
	defer resp.Body.Close()

	// Handle response from GitHub API
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get repository: %s", resp.Status)
	}

	// Decode the JSON response into a slice of Repository structs
	var githubResp []models.Organization
	if err := json.NewDecoder(resp.Body).Decode(&githubResp); err != nil {
		return nil, fmt.Errorf("failed to decode response body: %w", err)
	}

	return githubResp, nil
}

func GetOrgMembersAdminOnly(ctx *gin.Context) {
	orgID := ctx.Param("id")

	// Define a slice of structs to hold multiple members
	var members []struct {
		ProjectName string  `json:"project_name"`
		GithubName  *string `json:"github_name"`
		Role        *string `json:"role"`
	}

	// Execute the query
	rows, err := initializer.DB.Query(context.Background(), `
		SELECT DISTINCT
			u.github_name,
			p.name AS project_name,
			opum.role AS role
		FROM 
			users u
		JOIN 
			org_user_mapping oum ON u.id = oum.user_id
		LEFT JOIN 
			org_project_user_mapping opum ON u.id = opum.user_id AND oum.org_id = opum.org_id
		LEFT JOIN 
			projects p ON opum.project_id = p.id
		WHERE 
			oum.org_id = $1
	`, orgID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve members"})
		return
	}
	defer rows.Close()

	// Iterate over the result set
	for rows.Next() {
		var member struct {
			ProjectName string  `json:"project_name"`
			GithubName  *string `json:"github_name"`
			Role        *string `json:"role"`
		}

		// Initialize fields with default values in case of NULLs
		var githubName, role sql.NullString
		var projectName sql.NullString

		// Scan each row into temporary variables
		if err := rows.Scan(&githubName, &projectName, &role); err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan member"})
			return
		}

		// Assign values, setting to `nil` if they are NULL
		if githubName.Valid {
			member.GithubName = &githubName.String
		} else {
			member.GithubName = nil
		}
		if projectName.Valid {
			member.ProjectName = projectName.String
		} else {
			member.ProjectName = ""
		}
		if role.Valid {
			member.Role = &role.String
		} else {
			member.Role = nil
		}

		// Append each member to the members slice
		members = append(members, member)
	}

	// Check for any errors encountered during iteration
	if err := rows.Err(); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Error retrieving members"})
		return
	}

	ctx.JSON(http.StatusOK, members)
}

func GetSubscriptionBillingDetails(ctx *gin.Context) {
	orgID := ctx.Param("id")

	var subDetails struct {
		SubName    string `json:"sub_name"`
		MaxCount   int    `json:"max_count"`
		SubID      string `json:"sub_id"`
		ActiveUser int    `json:"active_user"`
	}

	// Get the subscription details from the database
	err := initializer.DB.QueryRow(context.Background(), `
    SELECT subs_name, max_user, subscription_id , user_count FROM organizations WHERE id = $1
	`, orgID).Scan(&subDetails.SubName, &subDetails.MaxCount, &subDetails.SubID, &subDetails.ActiveUser)

	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve subscription details"})
		return
	}

	subs, err := getSubscriptionBillDetailsFromPaypal(subDetails.SubID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch subscription bill details"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"subscription_details":      subDetails,
		"subscription_bill_details": subs,
	})

}

// getSubscriptionBillDetailsFromPaypal fetches subscription details from PayPal for the given subscription ID.
func getSubscriptionBillDetailsFromPaypal(subID string) (map[string]interface{}, error) {
	// Set PayPal API URL with the subscription ID
	url := fmt.Sprintf("https://api-m.sandbox.paypal.com/v1/billing/subscriptions/%s", subID)

	// Create an HTTP client and request
	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Authorization", "Bearer "+utils.PaypalAccessToken)
	req.Header.Add("Content-Type", "application/json")

	// Send request
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("failed to retrieve subscription details")
	}

	// Parse JSON response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	return result, nil
}

func GetSubscriptionTransactions(ctx *gin.Context) {
	orgID := ctx.Param("id") // Get subscription ID from URL parameters
	startTime, endTime := getSixMonthsDateRange()

	var subID string
	err := initializer.DB.QueryRow(context.Background(), `SELECT subscription_id FROM organizations WHERE id = $1`, orgID).Scan(&subID)

	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve subscription ID"})
		return
	}

	// PayPal transactions API URL with date range
	url := fmt.Sprintf(
		"https://api-m.sandbox.paypal.com/v1/billing/subscriptions/%s/transactions?start_time=%s&end_time=%s",
		subID, startTime, endTime,
	)

	// Create HTTP client and request
	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create request"})
		return
	}

	req.Header.Add("Authorization", "Bearer "+utils.PaypalAccessToken)
	req.Header.Add("Content-Type", "application/json")

	// Send request to PayPal API
	resp, err := client.Do(req)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve transactions"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		ctx.JSON(resp.StatusCode, gin.H{"error": "Failed to retrieve transactions from PayPal"})
		return
	}

	// Parse JSON response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read response body"})
		return
	}

	var transactions map[string]interface{}
	if err := json.Unmarshal(body, &transactions); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse JSON response"})
		return
	}

	ctx.JSON(http.StatusOK, transactions)
}

// getSixMonthsDateRange calculates the date range from six months ago to the current date.
func getSixMonthsDateRange() (string, string) {
	now := time.Now().UTC()              // Get the current time in UTC
	start := now.AddDate(0, -6, 0).UTC() // Get the date six months ago in UTC

	return start.Format(time.RFC3339), now.Format(time.RFC3339)
}

// CancelPayPalSubscription handles the canceling of a PayPal subscription
func CancelPayPalSubscription(c *gin.Context) {

	var body struct {
		OrgID string `json:"org_id"`
	}

	err := c.ShouldBindJSON(&body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	var subID string
	err = initializer.DB.QueryRow(context.Background(), `SELECT subscription_id FROM organizations WHERE id = $1`, body.OrgID).Scan(&subID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve subscription ID"})
		return
	}

	// PayPal API endpoint for canceling subscriptions
	url := fmt.Sprintf("https://api-m.sandbox.paypal.com/v1/billing/subscriptions/%s/suspend", subID)

	cancelRequestBody, err := json.Marshal(map[string]interface{}{
		"reason": "Not satisfied with the service",
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to marshal cancel request"})
		return
	}

	// Set PayPal API authorization and content headers
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(cancelRequestBody))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create cancel request"})
		return
	}

	req.Header.Set("Authorization", "Bearer "+utils.PaypalAccessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Send the request to PayPal API
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to cancel subscription"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H{"error": "Failed to cancel subscription with PayPal"})
		return
	}
	// Check if the cancellation was successful
	c.JSON(http.StatusOK, gin.H{"message": "Subscription cancellation request sent to PayPal"})
}

func ActivatePayPalSubscription(c *gin.Context) {
	var body struct {
		OrgID string `json:"org_id"`
	}

	// Bind JSON to the request body
	err := c.ShouldBindJSON(&body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	// Retrieve subscription ID from the database
	var subID string
	err = initializer.DB.QueryRow(context.Background(), `SELECT subscription_id FROM organizations WHERE id = $1`, body.OrgID).Scan(&subID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve subscription ID"})
		return
	}

	// PayPal API endpoint for activating subscriptions
	url := fmt.Sprintf("https://api-m.sandbox.paypal.com/v1/billing/subscriptions/%s/activate", subID)

	// Prepare the request body to activate the subscription (no reason field)
	activationRequestBody, err := json.Marshal(map[string]interface{}{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to marshal activation request"})
		return
	}

	// Create the HTTP request to PayPal API
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(activationRequestBody))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create activation request"})
		return
	}

	// Set PayPal API authorization and content headers
	req.Header.Set("Authorization", "Bearer "+utils.PaypalAccessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Send the request to PayPal API
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to communicate with PayPal"})
		return
	}
	defer resp.Body.Close()

	// Check PayPal response status
	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H{"error": "Failed to activate subscription with PayPal"})
		return
	}

	// Successfully activated the subscription
	c.JSON(http.StatusOK, gin.H{"message": "Subscription activated successfully"})
}
