package utils

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html/template"
	"log"
	"os"
	"time"

	"github.com/mailgun/mailgun-go/v4"
)

// MailgunClient is a global variable for the Mailgun client
var MailgunClient *mailgun.MailgunImpl
var SenderEmail string

// InitializeMailgun initializes the Mailgun client with the given configuration
func InitializeMailgun() {
	MailgunClient = mailgun.NewMailgun(os.Getenv("MAILGUN_EMAIL_DOMAIN"), os.Getenv("MAILGUN_API_KEY"))
	SenderEmail = "documentthing@gmail.com"
}

// SendOTPEmail sends an email with the OTP to the specified email address
func SendOTPEmail(email, otp, name string) error {
	if MailgunClient == nil {
		return errors.New("mailgun client not initialized")
	}

	// Parse the email template
	tmpl, err := template.ParseFiles("d:/Blocknotes/simpledocs-backend/internals/email-templates/otp.html")
	if err != nil {
		return fmt.Errorf("error parsing template: %w", err)
	}

	data := map[string]interface{}{
		"Name":      name,
		"OtpDigits": otp,
		"Year":      time.Now().Year(),
	}

	// Render the template with dynamic data
	var bodyBuffer bytes.Buffer
	if err := tmpl.Execute(&bodyBuffer, data); err != nil {
		return fmt.Errorf("error rendering template: %w", err)
	}

	// Create the email message
	subject := "Email Verification Code"
	message := mailgun.NewMessage(SenderEmail, subject, "", email)
	message.SetHTML(bodyBuffer.String()) // Set the HTML body

	// Set a timeout for the API call
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Send the email
	_, _, err = MailgunClient.Send(ctx, message)
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	log.Printf("OTP sent to %s successfully", email)
	return nil
}
