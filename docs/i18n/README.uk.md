# odotoken — Українська

Встановіть odotoken, спрямуйте на нього CLI і відкрийте панель.

> Translation pending native review. The English README is authoritative.

## Встановлення

Готовий бінарник у [Releases](https://github.com/ens-Emilio/odotoken/releases/latest) або з вихідного коду: `go build -o bin/odotoken ./cmd/odotoken`.

## Налаштування

`cp config.json.example config.json`, потім задайте `upstream_url` для кожного провайдера.

## Спрямувати CLI

Вкажіть `http://127.0.0.1:8080` (наприклад, `ANTHROPIC_BASE_URL`).

## Відкрити панель

`odotoken start`, потім http://127.0.0.1:8080/app/.

---

Повна документація: [README.md](README.md).
