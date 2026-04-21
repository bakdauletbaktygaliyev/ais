package handler

import (
	"fmt"
	"net/smtp"
	"os"
	"strconv"
)

func sendVerificationEmail(toEmail, code string) error {
	host := os.Getenv("SMTP_HOST")
	portStr := os.Getenv("SMTP_PORT")
	user := os.Getenv("SMTP_USER")
	pass := os.Getenv("SMTP_PASS")
	from := os.Getenv("SMTP_FROM")
	if from == "" {
		from = user
	}

	if host == "" || user == "" || pass == "" {
		return fmt.Errorf("SMTP not configured (set SMTP_HOST, SMTP_PORT, SMTP_USER, SMTP_PASS)")
	}

	port := 587
	if p, err := strconv.Atoi(portStr); err == nil && p > 0 {
		port = p
	}

	auth := smtp.PlainAuth("", user, pass, host)

	subject := "Your AIS verification code"
	body := fmt.Sprintf(
		"Your verification code for Architecture Insight System:\n\n  %s\n\nThis code expires in 15 minutes.\n",
		code,
	)
	msg := fmt.Sprintf(
		"From: AIS <%s>\r\nTo: %s\r\nSubject: %s\r\n\r\n%s",
		from, toEmail, subject, body,
	)

	addr := fmt.Sprintf("%s:%d", host, port)
	return smtp.SendMail(addr, auth, from, []string{toEmail}, []byte(msg))
}
