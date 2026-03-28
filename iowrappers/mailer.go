package iowrappers

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"

	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
	"github.com/weihesdlegend/Vacation-planner/user"
)

type Mailer struct {
	client      *sendgrid.Client
	redisClient *RedisClient
	email       string
}

type EmailType string

const EmailVerification EmailType = "Email Verification"
const PasswordReset EmailType = "Password Reset"

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

// Broadcast sends a message to all users
func (m *Mailer) Broadcast(ctx context.Context, subject, message, environment string) error {
	switch environment {
	case "development":
		return errors.New("cannot broadcast messages in dev environment")
	// TODO: Enable email capability in staging
	case "staging":
		return errors.New("cannot broadcast messages in staging environment")
	default:
		userEmails, err := m.redisClient.Get().HGetAll(ctx, UserEmailsKey).Result()
		if err != nil {
			return err
		}
		Logger.Debugf("obtained %d users from Redis", len(userEmails))

		from := mail.NewEmail("Vacation Planner", m.email)
		message = fmt.Sprintf("<div><p>Dear Users,<br><br>%s</p><div>", message)

		var sentCounter atomic.Int64
		wg := &sync.WaitGroup{}
		wg.Add(len(userEmails))
		for email := range userEmails {
			go func(e string) {
				defer wg.Done()
				msg := mail.NewSingleEmail(from, subject, mail.NewEmail(e, e), "", message)
				if _, err = m.client.Send(msg); err != nil {
					Logger.Error(err)
				}
				sentCounter.Add(1)
			}(email)
		}
		wg.Wait()
		Logger.Debugf("successfully sent emails to %d users", sentCounter.Load())
	}
	return nil
}

func (m *Mailer) Send(ctx context.Context, t EmailType, recipient user.View, environment string) error {
	Logger.Infof("->Mailer.Send: received request to send email for user %v", recipient)
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
		domain := os.Getenv("DOMAIN")
		if domain == "" {
			domain = "http://localhost:10000"
		}
		htmlContent := fmt.Sprintf("<p>please follow the <a href=%s/v1/verify?code=%s>link</a> to verify your email address. </p>", domain, code)
		message := mail.NewSingleEmail(from, subject, to, "", htmlContent)
		resp, err := m.client.Send(message)
		if err != nil {
			return err
		}
		if resp.StatusCode > 299 {
			return fmt.Errorf("failed to Send email, status %d", resp.StatusCode)
		}
	case PasswordReset:
		email := recipient.Email
		subject := "Password Reset"
		from := mail.NewEmail("Vacation Planner", m.email)
		to := mail.NewEmail(email, email)
		code, err := m.redisClient.saveEmailPasswordResetCode(ctx, recipient)
		if err != nil {
			return err
		}
		domain := os.Getenv("DOMAIN")
		if domain == "" {
			domain = "http://localhost:10000"
		}
		htmlContent := fmt.Sprintf("<p>please follow the <a href=%s/v1/reset-password?email=%s&code=%s>link</a> to reset your password. </p>", domain, recipient.Email, code)
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
