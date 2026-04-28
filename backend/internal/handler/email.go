package handler

import (
	"fmt"
	"io"
	"net/http"
	"net/smtp"
	"os"
	"strconv"
	"strings"
	"time"
)

func sendVerificationEmail(toEmail, code string) error {
	if key := os.Getenv("MAILJET_API_KEY"); key != "" {
		return sendViaMailjet(key, os.Getenv("MAILJET_SECRET_KEY"), toEmail, code)
	}
	return sendViaSMTP(toEmail, code)
}

func sendViaMailjet(apiKey, secretKey, toEmail, code string) error {
	from := os.Getenv("SMTP_FROM")
	text := fmt.Sprintf(
		"Your verification code for Architecture Insight System:\n\n  %s\n\nThis code expires in 15 minutes.",
		code,
	)
	body := fmt.Sprintf(`{
		"Messages":[{
			"From":{"Email":%q,"Name":"AIS"},
			"To":[{"Email":%q}],
			"Subject":"Your AIS verification code",
			"TextPart":%q
		}]
	}`, from, toEmail, text)

	req, err := http.NewRequest("POST", "https://api.mailjet.com/v3.1/send", strings.NewReader(body))
	if err != nil {
		return fmt.Errorf("mailjet request: %w", err)
	}
	req.SetBasicAuth(apiKey, secretKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return fmt.Errorf("mailjet send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("mailjet API %d: %s", resp.StatusCode, b)
	}
	return nil
}

func sendViaSMTP(toEmail, code string) error {
	host := os.Getenv("SMTP_HOST")
	portStr := os.Getenv("SMTP_PORT")
	user := os.Getenv("SMTP_USER")
	pass := os.Getenv("SMTP_PASS")
	from := os.Getenv("SMTP_FROM")
	if from == "" {
		from = user
	}

	if host == "" || user == "" || pass == "" {
		return fmt.Errorf("SMTP not configured")
	}

	port := 587
	if p, err := strconv.Atoi(portStr); err == nil && p > 0 {
		port = p
	}

	subject := "Your AIS verification code"
	text := fmt.Sprintf(
		"Your verification code for Architecture Insight System:\n\n  %s\n\nThis code expires in 15 minutes.\n",
		code,
	)
	msg := fmt.Sprintf("From: AIS <%s>\r\nTo: %s\r\nSubject: %s\r\n\r\n%s", from, toEmail, subject, text)

	auth := smtp.PlainAuth("", user, pass, host)
	return smtp.SendMail(fmt.Sprintf("%s:%d", host, port), auth, from, []string{toEmail}, []byte(msg))
}
