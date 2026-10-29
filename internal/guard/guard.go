package guard

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/smtp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ens-Emilio/odotoken/internal/config"
	"github.com/ens-Emilio/odotoken/internal/db"
	"github.com/ens-Emilio/odotoken/internal/domain"
)

// Guard enforces budget limits.
type Guard struct {
	db     *db.Database
	alerts config.AlertsConfig
	logger *slog.Logger

	limitsMu     sync.RWMutex
	limits       config.GuardConfig
	alertMu      sync.Mutex
	lastAlertKey string
	httpClient   *http.Client
}

// New creates a new Guard.
func New(database *db.Database, limits config.GuardConfig, alerts config.AlertsConfig, logger *slog.Logger) *Guard {
	return &Guard{
		db:         database,
		limits:     limits,
		alerts:     alerts,
		logger:     logger,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// Limits returns the current budget limits.
func (g *Guard) Limits() config.GuardConfig {
	g.limitsMu.RLock()
	defer g.limitsMu.RUnlock()
	return g.limits
}

// UpdateLimits replaces the in-memory limits. Persistence is owned by the
// caller, which also knows the config path.
func (g *Guard) UpdateLimits(limits config.GuardConfig) {
	g.limitsMu.Lock()
	g.limits = limits
	g.limitsMu.Unlock()
	g.alertMu.Lock()
	g.lastAlertKey = ""
	g.alertMu.Unlock()
}

// Check verifies if a request is allowed under current budget limits.
//
// A failure to read the budget fails OPEN: the request is allowed and the
// error is logged. The guard meters spend; it does not protect an upstream, so
// a SQLite hiccup must not take the proxy down for every caller. If a hard
// stop on unreadable budgets is ever wanted, it belongs behind an explicit
// config switch rather than as an implicit consequence of a disk error.
func (g *Guard) Check(projectID string) domain.GuardCheckResult {
	limits := g.Limits()
	loc, err := time.LoadLocation(limits.Timezone)
	if err != nil {
		if limits.Timezone != "" {
			g.logger.Warn("invalid guard timezone, using UTC", "timezone", limits.Timezone, "error", err)
		}
		loc = time.UTC
	}
	summary, err := g.db.GetCostSummaryInLocation(projectID, loc)
	if err != nil {
		g.logger.Error("guard check failed, allowing request (fail-open)", "error", err)
		return domain.GuardCheckResult{Allowed: true, Status: "ok"}
	}

	result := domain.GuardCheckResult{
		Allowed:             true,
		Status:              "ok",
		CurrentDailyCost:    summary.DailyCostUSD,
		DailyLimit:          limits.DailyCostLimitUSD,
		CurrentMonthlyCost:  summary.MonthlyCostUSD,
		MonthlyLimit:        limits.MonthlyCostLimitUSD,
		CurrentDailyTokens:  summary.DailyTokens,
		CurrentMonthTokens:  summary.MonthlyTokens,
		DailyTokensLimit:    limits.DailyTokenLimit,
		MonthlyTokensLimit:  limits.MonthlyTokenLimit,
		WarningThresholdPct: limits.WarningThresholdPct,
		ActionOnBreach:      limits.ActionOnBreach,
	}

	warnPct := limits.WarningThresholdPct
	if warnPct <= 0 {
		warnPct = 80.0
	}

	// Cost limits. Each branch records a stable code plus its numbers, because
	// the dashboard must render the reason in the reader's language; Message
	// stays English for logs and alert payloads, which no UI reads.
	reason := func(code, message string, params map[string]string) {
		result.ReasonCode = code
		result.ReasonParams = params
		result.Message = message
	}

	if limits.DailyCostLimitUSD > 0 && summary.DailyCostUSD >= limits.DailyCostLimitUSD {
		result.Allowed = false
		result.Status = "breached"
		reason("guard.dailyCostBreached",
			fmt.Sprintf("daily cost limit breached: $%.4f / $%.2f", summary.DailyCostUSD, limits.DailyCostLimitUSD),
			costParams(summary.DailyCostUSD, limits.DailyCostLimitUSD, 0))
	} else if limits.MonthlyCostLimitUSD > 0 && summary.MonthlyCostUSD >= limits.MonthlyCostLimitUSD {
		result.Allowed = false
		result.Status = "breached"
		reason("guard.monthlyCostBreached",
			fmt.Sprintf("monthly cost limit breached: $%.4f / $%.2f", summary.MonthlyCostUSD, limits.MonthlyCostLimitUSD),
			costParams(summary.MonthlyCostUSD, limits.MonthlyCostLimitUSD, 0))
	} else if limits.DailyCostLimitUSD > 0 && summary.DailyCostUSD >= limits.DailyCostLimitUSD*warnPct/100 {
		result.Status = "warning"
		reason("guard.dailyCostWarning",
			fmt.Sprintf("daily cost warning: $%.4f / $%.2f (%.0f%%)", summary.DailyCostUSD, limits.DailyCostLimitUSD, warnPct),
			costParams(summary.DailyCostUSD, limits.DailyCostLimitUSD, warnPct))
	} else if limits.MonthlyCostLimitUSD > 0 && summary.MonthlyCostUSD >= limits.MonthlyCostLimitUSD*warnPct/100 {
		result.Status = "warning"
		reason("guard.monthlyCostWarning",
			fmt.Sprintf("monthly cost warning: $%.4f / $%.2f (%.0f%%)", summary.MonthlyCostUSD, limits.MonthlyCostLimitUSD, warnPct),
			costParams(summary.MonthlyCostUSD, limits.MonthlyCostLimitUSD, warnPct))
	}

	// Token limits (only if not already breached by cost)
	if result.Status != "breached" {
		if limits.DailyTokenLimit > 0 && summary.DailyTokens >= limits.DailyTokenLimit {
			result.Allowed = false
			result.Status = "breached"
			reason("guard.dailyTokensBreached",
				fmt.Sprintf("daily token limit breached: %d / %d", summary.DailyTokens, limits.DailyTokenLimit),
				tokenParams(summary.DailyTokens, limits.DailyTokenLimit, 0))
		} else if limits.MonthlyTokenLimit > 0 && summary.MonthlyTokens >= limits.MonthlyTokenLimit {
			result.Allowed = false
			result.Status = "breached"
			reason("guard.monthlyTokensBreached",
				fmt.Sprintf("monthly token limit breached: %d / %d", summary.MonthlyTokens, limits.MonthlyTokenLimit),
				tokenParams(summary.MonthlyTokens, limits.MonthlyTokenLimit, 0))
		} else if limits.DailyTokenLimit > 0 && float64(summary.DailyTokens) >= float64(limits.DailyTokenLimit)*warnPct/100 {
			result.Status = "warning"
			reason("guard.dailyTokensWarning",
				fmt.Sprintf("daily token warning: %d / %d (%.0f%%)", summary.DailyTokens, limits.DailyTokenLimit, warnPct),
				tokenParams(summary.DailyTokens, limits.DailyTokenLimit, warnPct))
		} else if limits.MonthlyTokenLimit > 0 && float64(summary.MonthlyTokens) >= float64(limits.MonthlyTokenLimit)*warnPct/100 {
			result.Status = "warning"
			reason("guard.monthlyTokensWarning",
				fmt.Sprintf("monthly token warning: %d / %d (%.0f%%)", summary.MonthlyTokens, limits.MonthlyTokenLimit, warnPct),
				tokenParams(summary.MonthlyTokens, limits.MonthlyTokenLimit, warnPct))
		}
	}
	if result.Status == "breached" {
		result.Allowed = result.ActionOnBreach != "block"
	}

	// Alert only on state transition (ok -> warning/breached, or warning -> breached)
	if result.Status != "ok" {
		alertKey := result.Status + ":" + result.Message
		g.alertMu.Lock()
		if alertKey != g.lastAlertKey {
			g.lastAlertKey = alertKey
			g.alertMu.Unlock()
			if err := g.db.InsertGuardEvent(db.GuardEvent{
				Timestamp:      time.Now().UTC().Format(time.RFC3339Nano),
				Status:         result.Status,
				Message:        result.Message,
				MessageCode:    result.ReasonCode,
				MessageParams:  marshalParams(result.ReasonParams),
				Action:         result.ActionOnBreach,
				DailyCostUSD:   result.CurrentDailyCost,
				MonthlyCostUSD: result.CurrentMonthlyCost,
			}); err != nil {
				g.logger.Warn("could not persist guard transition", "error", err)
			}
			go g.dispatchAlerts(result)
		} else {
			g.alertMu.Unlock()
		}
	} else {
		g.alertMu.Lock()
		g.lastAlertKey = ""
		g.alertMu.Unlock()
	}

	return result
}

func costParams(used, limit, pct float64) map[string]string {
	return map[string]string{
		"used":  fmt.Sprintf("$%.2f", used),
		"limit": fmt.Sprintf("$%.2f", limit),
		"pct":   fmt.Sprintf("%.0f%%", pct),
	}
}

func tokenParams(used, limit int, pct float64) map[string]string {
	return map[string]string{
		"used":  strconv.Itoa(used),
		"limit": strconv.Itoa(limit),
		"pct":   fmt.Sprintf("%.0f%%", pct),
	}
}

// marshalParams stores reason parameters as JSON so the dashboard can rebuild
// the sentence in the reader's language long after the transition happened.
func marshalParams(params map[string]string) string {
	if len(params) == 0 {
		return "{}"
	}
	encoded, err := json.Marshal(params)
	if err != nil {
		return "{}"
	}
	return string(encoded)
}

func (g *Guard) dispatchAlerts(result domain.GuardCheckResult) {
	text := formatAlertText(result)

	if g.alerts.SlackURL != "" {
		g.sendWebhook(g.alerts.SlackURL, text)
	}
	if g.alerts.DiscordURL != "" {
		g.sendWebhook(g.alerts.DiscordURL, text)
	}
	if g.alerts.Telegram.BotToken != "" && g.alerts.Telegram.ChatID != "" {
		g.sendTelegram(g.alerts.Telegram, text)
	}
	if g.alerts.Email.SMTPHost != "" {
		g.sendEmail(g.alerts.Email, text)
	}
}

func formatAlertText(r domain.GuardCheckResult) string {
	icon := "⚠️"
	if r.Status == "breached" {
		icon = "🚨"
	}
	return fmt.Sprintf("%s odotoken alert: %s\nDaily: $%.4f | Monthly: $%.4f | DailyTokens: %d | MonthlyTokens: %d",
		icon, r.Message, r.CurrentDailyCost, r.CurrentMonthlyCost, r.CurrentDailyTokens, r.CurrentMonthTokens)
}

func (g *Guard) sendWebhook(url, text string) {
	resp, err := g.httpClient.Post(url, "application/json", strings.NewReader(fmt.Sprintf(`{"text":%q}`, text)))
	if err != nil {
		g.logger.Warn("webhook alert failed", "url", url, "error", err)
		return
	}
	resp.Body.Close()
}

func (g *Guard) sendTelegram(cfg config.TelegramConfig, text string) {
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", cfg.BotToken)
	body := fmt.Sprintf(`{"chat_id":%q,"text":%q}`, cfg.ChatID, text)
	resp, err := g.httpClient.Post(apiURL, "application/json", strings.NewReader(body))
	if err != nil {
		g.logger.Warn("telegram alert failed", "error", err)
		return
	}
	resp.Body.Close()
}

func (g *Guard) sendEmail(cfg config.EmailConfig, text string) {
	addr := net.JoinHostPort(cfg.SMTPHost, strconv.Itoa(cfg.SMTPPort))
	auth := smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.SMTPHost)
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: odotoken alert\r\n\r\n%s", cfg.FromAddr, cfg.ToAddr, text)
	err := smtp.SendMail(addr, auth, cfg.FromAddr, []string{cfg.ToAddr}, []byte(msg))
	if err != nil {
		g.logger.Warn("email alert failed", "error", err)
	}
}
