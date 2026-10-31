package guard

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/ens-Emilio/odotoken/internal/config"
	"github.com/ens-Emilio/odotoken/internal/domain"
)

// alertPayload is what the outbox stores per notification. It is JSON so a
// queued item stays readable after a restart, and so the worker never has to
// rebuild the message from the guard state, which may already have changed.
type alertPayload struct {
	Text string `json:"text"`
}

// alertChannel is one configured destination.
type alertChannel struct {
	name   string
	target string
}

// alertChannels lists every destination that is actually configured. An
// unconfigured channel is skipped instead of queued and retried forever.
func (g *Guard) alertChannels() []alertChannel {
	var out []alertChannel
	if g.alerts.WebhookURL != "" {
		out = append(out, alertChannel{"webhook", g.alerts.WebhookURL})
	}
	if g.alerts.SlackURL != "" {
		out = append(out, alertChannel{"slack", g.alerts.SlackURL})
	}
	if g.alerts.DiscordURL != "" {
		out = append(out, alertChannel{"discord", g.alerts.DiscordURL})
	}
	if g.alerts.Telegram.BotToken != "" && g.alerts.Telegram.ChatID != "" {
		out = append(out, alertChannel{"telegram", g.alerts.Telegram.ChatID})
	}
	if g.alerts.Email.SMTPHost != "" && g.alerts.Email.ToAddr != "" {
		out = append(out, alertChannel{"email", g.alerts.Email.ToAddr})
	}
	return out
}

// enqueueAlerts persists one row per channel. Nothing is sent inline: a
// webhook that is down at breach time must not lose the alert, and the caller
// (a proxied request) must not wait on SMTP.
func (g *Guard) enqueueAlerts(result domain.GuardCheckResult) {
	text := formatAlertText(result)
	body, err := json.Marshal(alertPayload{Text: text})
	if err != nil {
		g.logger.Warn("could not encode alert", "error", err)
		return
	}
	now := time.Now().UTC()
	for _, channel := range g.alertChannels() {
		if err := g.db.EnqueueAlert(channel.name, channel.target, string(body), now); err != nil {
			g.logger.Warn("could not queue alert", "channel", channel.name, "error", err)
		}
	}
	g.wakeWorker()
}

// wakeWorker nudges the retry loop so a fresh alert is attempted immediately
// rather than at the next tick. The buffer is one: a pending wake already
// covers every alert queued before it.
func (g *Guard) wakeWorker() {
	if g.wake == nil {
		return
	}
	select {
	case g.wake <- struct{}{}:
	default:
	}
}

// StartAlertWorker delivers queued alerts until ctx is cancelled. It also
// retries anything left over from a previous process, which is the point of
// keeping the queue in SQLite.
func (g *Guard) StartAlertWorker(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 5 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		g.deliverPending(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		case <-g.wake:
		}
	}
}

// DeliverPending runs one delivery pass and returns how many alerts were
// accepted. It is exported so `odotoken doctor --flush-alerts` can drain the
// queue without starting the daemon.
func (g *Guard) DeliverPending(ctx context.Context) int {
	return g.deliverPending(ctx)
}

func (g *Guard) deliverPending(ctx context.Context) int {
	maxAttempts := g.alerts.Retry.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 5
	}
	if _, err := g.db.DropExhaustedAlerts(maxAttempts); err != nil {
		g.logger.Warn("could not prune exhausted alerts", "error", err)
	}
	items, err := g.db.PendingAlerts(50)
	if err != nil {
		g.logger.Warn("could not read alert queue", "error", err)
		return 0
	}
	delivered := 0
	for _, item := range items {
		if ctx.Err() != nil {
			return delivered
		}
		var payload alertPayload
		if err := json.Unmarshal([]byte(item.Payload), &payload); err != nil {
			// A row this process cannot read is not retryable; retire it so it
			// cannot block the queue head forever.
			_ = g.db.MarkAlertFailed(item.ID, maxAttempts, time.Now().UTC(), "unreadable payload")
			continue
		}
		if err := g.deliver(ctx, item.Channel, item.Target, payload.Text); err != nil {
			attempts := item.Attempts + 1
			delay := g.retryDelay(attempts)
			g.logger.Warn("alert delivery failed",
				"channel", item.Channel, "attempt", attempts, "retry_in", delay.String(), "error", err)
			_ = g.db.MarkAlertFailed(item.ID, attempts, time.Now().UTC().Add(delay), err.Error())
			continue
		}
		if err := g.db.MarkAlertDelivered(item.ID); err != nil {
			g.logger.Warn("could not close alert", "id", item.ID, "error", err)
			continue
		}
		delivered++
	}
	return delivered
}

// retryDelay is exponential with a ceiling, so a long outage does not turn
// into a tight retry loop against the same failing endpoint.
func (g *Guard) retryDelay(attempts int) time.Duration {
	base := g.alerts.Retry.BaseDelaySeconds
	if base <= 0 {
		base = 30
	}
	maxDelay := g.alerts.Retry.MaxDelaySeconds
	if maxDelay <= 0 {
		maxDelay = 3600
	}
	seconds := float64(base) * math.Pow(2, float64(attempts-1))
	if seconds > float64(maxDelay) {
		seconds = float64(maxDelay)
	}
	return time.Duration(seconds) * time.Second
}

func (g *Guard) deliver(ctx context.Context, channel, target, text string) error {
	switch channel {
	case "webhook":
		return g.postGenericWebhook(ctx, target, text)
	case "slack", "discord":
		return g.postWebhook(ctx, target, text)
	case "telegram":
		return g.postTelegram(ctx, text)
	case "email":
		return g.sendEmail(g.alerts.Email, text)
	default:
		return fmt.Errorf("unknown alert channel %q", channel)
	}
}

// RetrySettings exposes the configured backoff for doctor output.
func (g *Guard) RetrySettings() config.AlertRetry {
	return g.alerts.Retry
}
