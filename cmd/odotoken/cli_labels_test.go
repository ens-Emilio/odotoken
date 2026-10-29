package main

import "testing"

// The dashboard ships twenty languages; the CLI must not fall back to English
// for any of them. A missing entry here is a silent regression: the terminal
// simply answers in English and nothing else notices.
func TestEveryDashboardLocaleHasCliLabels(t *testing.T) {
	locales := []string{
		"en", "pt-BR", "es", "zh-CN", "zh-TW", "ja", "ko", "de", "fr", "it",
		"ru", "uk", "pl", "tr", "hi", "id", "vi", "th", "ar", "fa",
	}
	for _, locale := range locales {
		labels := cliLabelsFor(locale)
		if labels == (cliLabels{}) {
			t.Errorf("%s: no labels", locale)
			continue
		}
		// Every field is printed; a blank one would leave a hole in the table.
		for name, value := range map[string]string{
			"status": labels.status, "files": labels.files, "missing": labels.missing,
			"ready": labels.ready, "scanned": labels.scanned, "new": labels.new,
			"dupes": labels.dupes, "cost": labels.cost, "total": labels.total,
			"tokens": labels.tokens, "unpriced": labels.unpriced,
			"importWord": labels.importWord, "dryRun": labels.dryRun,
		} {
			if value == "" {
				t.Errorf("%s: %s is empty", locale, name)
			}
		}
	}
}

// A language tag from the terminal arrives as `ru-RU`, `pt_BR.UTF-8` or plain
// `de`; all three have to reach the catalog instead of English.
func TestCliLanguageReadsTheTerminalLocale(t *testing.T) {
	t.Setenv("LC_ALL", "")
	t.Setenv("LC_MESSAGES", "")
	for _, tc := range []struct{ env, want string }{
		{"ru_RU.UTF-8", "ru"},
		{"pt_BR.UTF-8", "pt-BR"},
		{"de_DE.UTF-8", "de"},
		{"zh_TW.UTF-8", "zh-TW"},
		{"C", "en"},
		{"POSIX", "en"},
		{"", "en"},
	} {
		t.Setenv("LANG", tc.env)
		if got := cliLanguage(""); got != tc.want {
			t.Errorf("LANG=%q: got %q, want %q", tc.env, got, tc.want)
		}
	}

	// An explicit flag outranks the terminal, and a language tag without a
	// region still resolves.
	t.Setenv("LANG", "ru_RU.UTF-8")
	if got := cliLanguage("ja"); got != "ja" {
		t.Errorf("--lang must win: got %q", got)
	}
	if labels := cliLabelsFor("pt"); labels != cliLabelsByLocale["pt-BR"] {
		t.Error("bare `pt` must resolve to pt-BR")
	}
}
