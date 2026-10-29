# odotoken — فارسی

odotoken را نصب کنید، CLI خود را به آن وصل کنید و داشبورد را باز کنید.

> Translation pending native review. The English README is authoritative.

## نصب

فایل اجرایی آماده در [Releases](https://github.com/ens-Emilio/odotoken/releases/latest)، یا از کد منبع: `go build -o bin/odotoken ./cmd/odotoken`.

## پیکربندی

`cp config.json.example config.json` و سپس `upstream_url` هر ارائه‌دهنده را تنظیم کنید.

## اتصال CLI

آن را به `http://127.0.0.1:8080` وصل کنید (مثلاً `ANTHROPIC_BASE_URL`).

## باز کردن داشبورد

`odotoken start` و سپس http://127.0.0.1:8080/app/.

---

مستندات کامل: [README.md](README.md).
