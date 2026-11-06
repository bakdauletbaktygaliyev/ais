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
	if key := os.Getenv("BREVO_API_KEY"); key != "" {
		return sendViaBrevo(key, toEmail, code)
	}
	if key := os.Getenv("RESEND_API_KEY"); key != "" {
		return sendViaResend(key, toEmail, code)
	}
	return sendViaSMTP(toEmail, code)
}

func sendViaBrevo(apiKey, toEmail, code string) error {
	from := os.Getenv("SMTP_FROM")
	if from == "" {
		from = "noreply@ais.app"
	}
	text := fmt.Sprintf(
		"Your verification code for Architecture Insight System:\n\n  %s\n\nThis code expires in 15 minutes.",
		code,
	)
	body := fmt.Sprintf(`{
		"sender":{"email":%q,"name":"AIS"},
		"to":[{"email":%q}],
		"subject":"Your AIS verification code",
		"textContent":%q
	}`, from, toEmail, text)

	req, err := http.NewRequest("POST", "https://api.brevo.com/v3/smtp/email", strings.NewReader(body))
	if err != nil {
		return fmt.Errorf("brevo request: %w", err)
	}
	req.Header.Set("api-key", apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return fmt.Errorf("brevo send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("brevo API %d: %s", resp.StatusCode, b)
	}
	return nil
}

func sendViaResend(apiKey, toEmail, code string) error {
	from := os.Getenv("SMTP_FROM")
	if from == "" {
		from = "onboarding@resend.dev"
	}
	text := fmt.Sprintf(
		"Your verification code for Architecture Insight System:\n\n  %s\n\nThis code expires in 15 minutes.",
		code,
	)
	body := fmt.Sprintf(`{"from":%q,"to":[%q],"subject":"Your AIS verification code","text":%q}`,
		from, toEmail, text)

	req, err := http.NewRequest("POST", "https://api.resend.com/emails", strings.NewReader(body))
	if err != nil {
		return fmt.Errorf("resend request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return fmt.Errorf("resend send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("resend API %d: %s", resp.StatusCode, b)
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
