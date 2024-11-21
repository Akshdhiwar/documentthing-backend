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
	fmt.Println()
}

// SendOTPEmail sends an email with the OTP to the specified email address
func SendOTPEmail(email, otp, name string) error {
	if MailgunClient == nil {
		return errors.New("mailgun client not initialized")
	}

	// Inline HTML template
	const htmlTemplate = `
	<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>Email Verification</title>
  </head>
  <body
    style="
      font-family: Arial, sans-serif;
      margin: 0;
      padding: 0;
      background-color: #f4f4f9;
      color: #333;
    "
  >
    <div
      style="
        max-width: 600px;
        margin: 0 auto;
        background: #ffffff;
        padding: 20px;
        border-radius: 8px;
        box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
      "
    >
      <div style="text-align: center; margin-bottom: 20px">
        <p
          style="
            color: var(--primary-color);
            font-size: 1.5rem;
            line-height: 2rem;
            font-weight: 200;
            letter-spacing: -0.05em;
            text-transform: none;
            margin: 0;
          "
        >
          <span style="font-weight: 600">document</span>Thing
        </p>
      </div>

      <div>
        <h2 style="color: #4a4a4a">Hi {{.Name}},</h2>
        <p>Thank you for signing up with DocumentThing!</p>
        <p>
          To verify your email address and activate your account, please use the
          One-Time Password (OTP) provided below. This OTP is valid for the next
          10 minutes:
        </p>
        <div style="margin-top: 20px; text-align: center">
          <div
            style="
              display: inline-block;
              text-align: center;
              line-height: 50px;
              border: 2px solid #007bff;
              color: #007bff;
              font-size: 24px;
              font-weight: bold;
              border-radius: 8px;
            "
          >
            {{.OtpDigits}}
          </div>
        </div>

        <p style="margin-top: 20px">
          If you didn’t request this, please ignore this email or contact our
          support team if you have concerns.
        </p>

        <p style="margin-top: 20px">We’re excited to have you on board!</p>
      </div>

      <div
        style="
          text-align: center;
          margin-top: 40px;
          font-size: 12px;
          color: #888;
        "
      >
        <p>
          <span style="font-weight: 600">Need Help?</span><br />
          If you face any issues, feel free to reach out to us at
          documentthing@gmail.com.
        </p>
        <p>© {{.Year}} Documentthing. All Rights Reserved.</p>
      </div>
    </div>
  </body>
</html>

	`

	// Parse the inline HTML template
	tmpl, err := template.New("otpEmail").Parse(htmlTemplate)
	if err != nil {
		return fmt.Errorf("error parsing inline template: %w", err)
	}

	// Data to render the template
	data := map[string]interface{}{
		"Name":      name,
		"OtpDigits": otp,
		"Year":      time.Now().Year(),
	}

	// Render the template
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
