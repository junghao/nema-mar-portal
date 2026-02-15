package email

import (
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net"
	"net/smtp"
	"os"
	"strings"

	"github.com/GeoNet/nema-mar-portal/internal/fastschema"
)

// Config holds SMTP configuration.
type Config struct {
	Host       string
	Port       string
	Username   string
	Password   string
	FromAddr   string
	Recipients []string
}

// ConfigFromEnv reads SMTP configuration from environment variables.
func ConfigFromEnv() (Config, error) {
	cfg := Config{
		Host:     os.Getenv("SMTP_HOST"),
		Port:     os.Getenv("SMTP_PORT"),
		Username: os.Getenv("SMTP_USERNAME"),
		Password: os.Getenv("SMTP_PASSWORD"),
		FromAddr: os.Getenv("SMTP_FROM"),
	}

	if cfg.Port == "" {
		cfg.Port = "587"
	}

	recipients := os.Getenv("SMTP_RECIPIENTS")
	if recipients != "" {
		cfg.Recipients = strings.Split(recipients, ",")
		for i := range cfg.Recipients {
			cfg.Recipients[i] = strings.TrimSpace(cfg.Recipients[i])
		}
	}

	if cfg.Host == "" {
		return cfg, fmt.Errorf("SMTP_HOST not set")
	}
	if cfg.FromAddr == "" {
		return cfg, fmt.Errorf("SMTP_FROM not set")
	}
	if len(cfg.Recipients) == 0 {
		return cfg, fmt.Errorf("SMTP_RECIPIENTS not set")
	}

	return cfg, nil
}

// SendEATEmail sends the EAT notification email with PDF attachment.
func SendEATEmail(cfg Config, eat *fastschema.EAT, pdfBytes []byte) error {
	subject := fmt.Sprintf("EAT: %s (Version %d) - %s", eat.EventTitle, eat.Version, eat.Status)

	body := fmt.Sprintf(`Emergency Advisory Text

Event: %s
Version: %d
Status: %s
Location: %s
Event Date: %s
Magnitude: %.1f
Beach/Marine Threat: %s
Land Threat: %s
TEP Activated: %s

Comments:
%s
`,
		eat.EventTitle,
		eat.Version,
		eat.Status,
		eat.Location,
		eat.EventDate.UTC().Format("2006-01-02 15:04 UTC"),
		eat.Magnitude,
		boolStr(eat.BeachMarineThreat),
		boolStr(eat.LandThreat),
		boolStr(eat.TEPActivated),
		eat.EventComments,
	)

	// Build MIME message
	boundary := "==NEMA_MAR_BOUNDARY=="
	var msg strings.Builder

	msg.WriteString(fmt.Sprintf("From: %s\r\n", cfg.FromAddr))
	msg.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(cfg.Recipients, ", ")))
	msg.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString(fmt.Sprintf("Content-Type: multipart/mixed; boundary=\"%s\"\r\n", boundary))
	msg.WriteString("\r\n")

	// Text body
	msg.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	msg.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
	msg.WriteString("Content-Transfer-Encoding: 7bit\r\n")
	msg.WriteString("\r\n")
	msg.WriteString(body)
	msg.WriteString("\r\n")

	// PDF attachment
	if pdfBytes != nil {
		msg.WriteString(fmt.Sprintf("--%s\r\n", boundary))
		msg.WriteString("Content-Type: application/pdf\r\n")
		msg.WriteString("Content-Transfer-Encoding: base64\r\n")
		msg.WriteString(fmt.Sprintf("Content-Disposition: attachment; filename=\"%s_v%d.pdf\"\r\n",
			eat.EventTitle, eat.Version))
		msg.WriteString("\r\n")
		msg.WriteString(base64.StdEncoding.EncodeToString(pdfBytes))
		msg.WriteString("\r\n")
	}

	msg.WriteString(fmt.Sprintf("--%s--\r\n", boundary))

	// Send via SMTP with STARTTLS
	addr := net.JoinHostPort(cfg.Host, cfg.Port)
	auth := smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return fmt.Errorf("dial smtp: %w", err)
	}

	client, err := smtp.NewClient(conn, cfg.Host)
	if err != nil {
		return fmt.Errorf("smtp client: %w", err)
	}
	defer client.Close()

	// Try STARTTLS
	if ok, _ := client.Extension("STARTTLS"); ok {
		tlsCfg := &tls.Config{ServerName: cfg.Host}
		if err := client.StartTLS(tlsCfg); err != nil {
			return fmt.Errorf("starttls: %w", err)
		}
	}

	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("smtp auth: %w", err)
	}

	if err := client.Mail(cfg.FromAddr); err != nil {
		return fmt.Errorf("smtp mail: %w", err)
	}

	for _, rcpt := range cfg.Recipients {
		if err := client.Rcpt(rcpt); err != nil {
			return fmt.Errorf("smtp rcpt %s: %w", rcpt, err)
		}
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}

	if _, err := w.Write([]byte(msg.String())); err != nil {
		return fmt.Errorf("smtp write: %w", err)
	}

	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp close: %w", err)
	}

	return client.Quit()
}

func boolStr(b bool) string {
	if b {
		return "Yes"
	}
	return "No"
}
