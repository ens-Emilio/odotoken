package guard

import (
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/smtp"
	"strconv"
	"strings"

	"github.com/user/odotoken/internal/config"
	"github.com/user/odotoken/internal/db"
	"github.com/user/odotoken/internal/domain"
)

// Guard enforces budget limits.
type Guard struct {
	db     *db.Database
	limits config.GuardConfig
	alerts config.AlertsConfig
	logger *slog.Logger
}

// New creates a new Guard.
func New(database *db.Database, limits config.GuardConfig, alerts config.AlertsConfig, logger *slog.Logger) *Guard {
	return &Guard{db: database, limits: limits, alerts: alerts, logger: logger}
}

// Check verifies if a request is allowed under current budget limits.
func (g *Guard) Check(projectID string) domain.GuardCheckResult {
	summary, err := g.db.GetCostSummary(projectID)
	if err != nil {
		g.logger.Error("guard check failed", "error", err)
		return domain.GuardCheckResult{Allowed: true, Status: "ok"}
	}

	result := domain.GuardCheckResult{
		Allowed:            true,
		Status:             "ok",
		CurrentDailyCost:   summary.DailyCostUSD,
		DailyLimit:         g.limits.DailyCostLimitUSD,
		CurrentMonthlyCost: summary.MonthlyCostUSD,
		MonthlyLimit:       g.limits.MonthlyCostLimitUSD,
		CurrentDailyTokens: summary.DailyTokens,
		CurrentMonthTokens: summary.MonthlyTokens,
	}

	warnPct := g.limits.WarningThresholdPct
	if warnPct <= 0 {
		warnPct = 80.0
	}

	if g.limits.DailyCostLimitUSD > 0 && summary.DailyCostUSD >= g.limits.DailyCostLimitUSD {
		result.Allowed = false
		result.Status = "breached"
		result.Message = fmt.Sprintf("daily cost limit breached: $%.4f / $%.2f", summary.DailyCostUSD, g.limits.DailyCostLimitUSD)
	} else if g.limits.MonthlyCostLimitUSD > 0 && summary.MonthlyCostUSD >= g.limits.MonthlyCostLimitUSD {
		result.Allowed = false
		result.Status = "breached"
		result.Message = fmt.Sprintf("monthly cost limit breached: $%.4f / $%.2f", summary.MonthlyCostUSD, g.limits.MonthlyCostLimitUSD)
	} else if g.limits.DailyCostLimitUSD > 0 && summary.DailyCostUSD >= g.limits.DailyCostLimitUSD*warnPct/100 {
		result.Status = "warning"
		result.Message = fmt.Sprintf("daily cost warning: $%.4f / $%.2f (%.0f%%)", summary.DailyCostUSD, g.limits.DailyCostLimitUSD, warnPct)
	} else if g.limits.MonthlyCostLimitUSD > 0 && summary.MonthlyCostUSD >= g.limits.MonthlyCostLimitUSD*warnPct/100 {
		result.Status = "warning"
		result.Message = fmt.Sprintf("monthly cost warning: $%.4f / $%.2f (%.0f%%)", summary.MonthlyCostUSD, g.limits.MonthlyCostLimitUSD, warnPct)
	}

	if result.Status != "ok" {
		go g.dispatchAlerts(result)
	}

	return result
}

func (g *Guard) dispatchAlerts(result domain.GuardCheckResult) {
	text := formatAlertText(result)

	if g.alerts.SlackURL != "" {
		sendWebhook(g.alerts.SlackURL, text)
	}
	if g.alerts.DiscordURL != "" {
		sendWebhook(g.alerts.DiscordURL, text)
	}
	if g.alerts.Telegram.BotToken != "" && g.alerts.Telegram.ChatID != "" {
		sendTelegram(g.alerts.Telegram, text)
	}
	if g.alerts.Email.SMTPHost != "" {
		sendEmail(g.alerts.Email, text)
	}
}

func formatAlertText(r domain.GuardCheckResult) string {
	icon := "⚠️"
	if r.Status == "breached" {
		icon = "🚨"
	}
	return fmt.Sprintf("%s odotoken alert: %s\nDaily: $%.4f | Monthly: $%.4f", icon, r.Message, r.CurrentDailyCost, r.CurrentMonthlyCost)
}

func sendWebhook(url, text string) {
	resp, err := http.Post(url, "application/json", strings.NewReader(fmt.Sprintf(`{"text":%q}`, text)))
	if err != nil {
		slog.Warn("webhook alert failed", "url", url, "error", err)
		return
	}
	resp.Body.Close()
}

func sendTelegram(cfg config.TelegramConfig, text string) {
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", cfg.BotToken)
	body := fmt.Sprintf(`{"chat_id":%q,"text":%q}`, cfg.ChatID, text)
	resp, err := http.Post(apiURL, "application/json", strings.NewReader(body))
	if err != nil {
		slog.Warn("telegram alert failed", "error", err)
		return
	}
	resp.Body.Close()
}

func sendEmail(cfg config.EmailConfig, text string) {
	addr := net.JoinHostPort(cfg.SMTPHost, strconv.Itoa(cfg.SMTPPort))
	auth := smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.SMTPHost)
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: odotoken alert\r\n\r\n%s", cfg.FromAddr, cfg.ToAddr, text)
	err := smtp.SendMail(addr, auth, cfg.FromAddr, []string{cfg.ToAddr}, []byte(msg))
	if err != nil {
		slog.Warn("email alert failed", "error", err)
	}
}