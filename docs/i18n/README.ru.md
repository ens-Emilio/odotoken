# odotoken — Русский

Установите odotoken, направьте на него CLI и откройте панель.

> Translation pending native review. The English README is authoritative.

## Установка

Готовый бинарник в [Releases](https://github.com/ens-Emilio/odotoken/releases/latest) или из исходников: `go build -o bin/odotoken ./cmd/odotoken`.

## Настройка

`cp config.json.example config.json`, затем укажите `upstream_url` для каждого провайдера.

## Направить CLI

Укажите `http://127.0.0.1:8080` (например, `ANTHROPIC_BASE_URL`).

## Открыть панель

`odotoken start`, затем http://127.0.0.1:8080/app/.

---

Полная документация: [README.md](README.md).
