package iowrappers

import (
	"context"
	"errors"
	"fmt"
	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
	"github.com/weihesdlegend/Vacation-planner/user"
	"os"
)

type Mailer struct {
	client      *sendgrid.Client
	redisClient *RedisClient
	email       string
}

type EmailType string

const EmailVerification EmailType = "Email Verification"

func (m *Mailer) Init(redisClient *RedisClient) error {
	if os.Getenv("SENDGRID_API_KEY") == "" {
		return errors.New("failed to create mailer: SENDGRID_API_KEY does not exist")
	}
	if os.Getenv("MAILER_EMAIL_ADDRESS") == "" {
		return errors.New("failed to create mailer: MAILER_EMAIL_ADDRESS cannot be empty")
	}
	m.client = sendgrid.NewSendClient(os.Getenv("SENDGRID_API_KEY"))
	m.redisClient = redisClient
	m.email = os.Getenv("MAILER_EMAIL_ADDRESS")
	return nil
}

func (m *Mailer) Send(t EmailType, recipient user.View) error {
	Logger.Infof("->Mailer.Send: received request to send email for user %v", recipient)
	ctx := context.Background()
	switch t {
	case EmailVerification:
		email := recipient.Email
		subject := "Email Verification"
		from := mail.NewEmail("Vacation Planner", m.email)
		to := mail.NewEmail(email, email)
		code, err := m.redisClient.saveUserEmailVerificationCode(ctx, recipient)
		if err != nil {
			return err
		}
		htmlContent := fmt.Sprintf("<p>please follow the <a href=https://www.unwind.dev/v1/verify?code=%s>link</a> to verify your email address. </p>", code)
		message := mail.NewSingleEmail(from, subject, to, "", htmlContent)
		resp, err := m.client.Send(message)
		if err != nil {
			return err
		}
		if resp.StatusCode > 299 {
			return fmt.Errorf("failed to Send email, status %d", resp.StatusCode)
		}
	default:
		return errors.New("mailer.Send: email type not implemented")
	}
	return nil
}