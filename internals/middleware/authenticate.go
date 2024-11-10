package middleware

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func AuthMiddleware(ctx *gin.Context) {
	// Extract the token from the cookie
	tokenString, err := ctx.Cookie("betterDocsAT")
	if err != nil {

		if tokenString == "" {
			generateNewAccessToken(ctx)
			ctx.Next()
			return
		}

		ctx.JSON(http.StatusUnauthorized, gin.H{
			"message": "Authorization token not found",
		})
		ctx.Abort()
		return
	}

	// Parse and validate the token
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Make sure the signing method is HMAC
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		// Return the secret signing key
		return []byte(os.Getenv("JWTSECRET_ACCESS")), nil
	})

	// Handle errors in token parsing
	if err != nil {
		if err.Error() == "token has invalid claims: token is expired" {
			generateNewAccessToken(ctx)
			ctx.Next()
			return
		}
		ctx.JSON(http.StatusUnauthorized, gin.H{
			"message": "Authorization token not found",
		})
		ctx.Abort()
		return
	}

	// Check if the token is valid and extract claims
	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		// Store the claims in the context
		ctx.Set("claims", claims)
	} else {
		generateNewAccessToken(ctx)
		ctx.Next()
		return
	}

	// Proceed to the next handler
	ctx.Next()
}

func AuthenticateSuperAdminApi(ctx *gin.Context) {
	// Extract the token from the cookie
	tokenString, err := ctx.Cookie("betterDocsAT")
	if err != nil {

		if tokenString == "" {
			generateNewAccessToken(ctx)
			ctx.Next()
			return
		}

		ctx.JSON(http.StatusUnauthorized, gin.H{
			"message": "Authorization token not found",
		})
		ctx.Abort()
		return
	}

	// Parse and validate the token
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Make sure the signing method is HMAC
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		// Return the secret signing key
		return []byte(os.Getenv("JWTSECRET_ACCESS")), nil
	})

	// Handle errors in token parsing
	if err != nil {
		if err.Error() == "token has invalid claims: token is expired" {
			generateNewAccessToken(ctx)
			ctx.Next()
			return
		}
		ctx.JSON(http.StatusUnauthorized, gin.H{
			"message": "Authorization token not found",
		})
		ctx.Abort()
		return
	}

	// Check if the token is valid and extract claims
	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		// Store the claims in the context
		if sub, ok := claims["sub"].(string); ok {
			if sub == os.Getenv("ADMIN_GITHUB_ID") {
				ctx.Next()
				return
			}
			ctx.Abort()
		}
	}

	// Proceed to the next handler
	ctx.Next()
}

func generateNewAccessToken(ctx *gin.Context) {
	// Extract the token from the cookie
	tokenString, err := ctx.Cookie("betterDocsRT")
	if err != nil {
		ctx.JSON(http.StatusUnauthorized, gin.H{
			"message": "Authorization token not found",
		})
		ctx.Abort()
		return
	}

	// Parse and validate the token
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Make sure the signing method is HMAC
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		// Return the secret signing key
		return []byte(os.Getenv("JWTSECRET_REFRESH")), nil
	})

	// Handle errors in token parsing
	if err != nil {
		ctx.JSON(http.StatusUnauthorized, gin.H{
			"message": "Invalid or expired token",
		})
		ctx.Abort()
		return
	}

	// Check if the token is valid and extract claims
	claims, ok := token.Claims.(jwt.MapClaims)
	if ok && token.Valid {
		// Store the claims in the context
		ctx.Set("claims", claims)
	} else {
		ctx.JSON(http.StatusUnauthorized, gin.H{
			"message": "Invalid token",
		})
		ctx.Abort()
		return
	}

	// Generate a new access token with the same claims
	newAccessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"id":       claims["id"],
		"username": claims["username"],
		"email":    claims["email"],
		"iat":      time.Now(),
		"exp":      time.Now().Add(time.Hour * 24).Unix(),
	})

	tokenString, err = newAccessToken.SignedString([]byte(os.Getenv("JWTSECRET_ACCESS")))
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"message": "Error generating new access token: " + err.Error()})
		return
	}

	ctx.SetCookie("betterDocsAT", tokenString, 60*60*24, "/", "", false, true) // Set the access token cookie with a 24-hour
}
