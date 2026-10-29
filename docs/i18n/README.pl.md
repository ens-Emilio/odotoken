# odotoken — Polski

Zainstaluj odotoken, skieruj CLI na niego i otwórz panel.

> Translation pending native review. The English README is authoritative.

## Instalacja

Gotowy plik binarny w [Releases](https://github.com/ens-Emilio/odotoken/releases/latest) lub ze źródeł: `go build -o bin/odotoken ./cmd/odotoken`.

## Konfiguracja

`cp config.json.example config.json`, następnie ustaw `upstream_url` dla każdego providera.

## Skieruj CLI

Wskaż `http://127.0.0.1:8080` (na przykład `ANTHROPIC_BASE_URL`).

## Otwórz panel

`odotoken start`, następnie http://127.0.0.1:8080/app/.

---

Pełna dokumentacja: [README.md](README.md).
