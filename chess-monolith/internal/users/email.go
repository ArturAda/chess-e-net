package users

import (
	"fmt"
	"log"
	"net"
	"net/mail"
	"net/smtp"
	"os"
	"strings"
)

// EmailSender abstracts delivery so tests and local development do not need SMTP.
type EmailSender interface {
	SendVerificationCode(to string, code string) error
}

type LogEmailSender struct{}

func NewLogEmailSender() EmailSender {
	return LogEmailSender{}
}

func (LogEmailSender) SendVerificationCode(to string, code string) error {
	log.Printf("[EMAIL VERIFICATION] code for %s: %s", to, code)
	return nil
}

type SMTPEmailSender struct {
	Host     string
	Port     string
	Username string
	Password string
	From     string
}

func NewEmailSenderFromEnv() EmailSender {
	host := strings.TrimSpace(os.Getenv("SMTP_HOST"))
	from := strings.TrimSpace(os.Getenv("SMTP_FROM"))
	if host == "" || from == "" {
		log.Println("SMTP_HOST/SMTP_FROM are not configured; email verification codes will be logged locally")
		return NewLogEmailSender()
	}

	port := strings.TrimSpace(os.Getenv("SMTP_PORT"))
	if port == "" {
		port = "587"
	}

	return &SMTPEmailSender{
		Host:     host,
		Port:     port,
		Username: strings.TrimSpace(os.Getenv("SMTP_USERNAME")),
		Password: os.Getenv("SMTP_PASSWORD"),
		From:     from,
	}
}

func (s *SMTPEmailSender) SendVerificationCode(to string, code string) error {
	fromAddress, err := mail.ParseAddress(s.From)
	if err != nil {
		return fmt.Errorf("%w: invalid SMTP_FROM", ErrEmailDelivery)
	}

	toAddress, err := mail.ParseAddress(to)
	if err != nil {
		return fmt.Errorf("%w: invalid recipient", ErrEmailDelivery)
	}

	subject := "Chess-E-Net email verification"
	body := fmt.Sprintf("Your Chess-E-Net verification code is: %s\n\nThe code expires in 1 minute.\n", code)
	message := strings.Join([]string{
		"From: " + fromAddress.String(),
		"To: " + toAddress.String(),
		"Subject: " + subject,
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"",
		body,
	}, "\r\n")

	address := net.JoinHostPort(s.Host, s.Port)
	var auth smtp.Auth
	if s.Username != "" {
		auth = smtp.PlainAuth("", s.Username, s.Password, s.Host)
	}

	if err := smtp.SendMail(address, auth, fromAddress.Address, []string{toAddress.Address}, []byte(message)); err != nil {
		return fmt.Errorf("%w: %v", ErrEmailDelivery, err)
	}

	return nil
}
