package guard

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/smtp"
	"strconv"

	"github.com/ens-Emilio/odotoken/internal/config"
)

// postGenericWebhook posts a small odotoken JSON payload for generic receivers.
func (g *Guard) postGenericWebhook(ctx context.Context, target, text string) error {
	body, err := json.Marshal(map[string]string{"source": "odotoken", "text": text})
	if err != nil {
		return err
	}
	return g.postJSON(ctx, target, body)
}

// postWebhook sends a Slack/Discord-compatible JSON payload. A non-2xx answer
// is an error so the outbox retries it; silently ignoring the status is what
// made a revoked webhook look delivered.
func (g *Guard) postWebhook(ctx context.Context, target, text string) error {
	body, err := json.Marshal(map[string]string{"text": text})
	if err != nil {
		return err
	}
	return g.postJSON(ctx, target, body)
}

func (g *Guard) postTelegram(ctx context.Context, text string) error {
	cfg := g.alerts.Telegram
	target := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", cfg.BotToken)
	body, err := json.Marshal(map[string]string{"chat_id": cfg.ChatID, "text": text})
	if err != nil {
		return err
	}
	return g.postJSON(ctx, target, body)
}

func (g *Guard) postJSON(ctx context.Context, target string, body []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := g.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	detail, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	if resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(bytes.TrimSpace(detail)))
	}
	return nil
}

func (g *Guard) sendEmail(cfg config.EmailConfig, text string) error {
	port := cfg.SMTPPort
	if port <= 0 {
		port = 587
	}
	addr := net.JoinHostPort(cfg.SMTPHost, strconv.Itoa(port))
	auth := smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.SMTPHost)
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: odotoken alert\r\n\r\n%s",
		cfg.FromAddr, cfg.ToAddr, text)
	return smtp.SendMail(addr, auth, cfg.FromAddr, []string{cfg.ToAddr}, []byte(msg))
}
