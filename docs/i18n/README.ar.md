# odotoken — العربية

ثبّت odotoken، ووجّه CLI إليه، ثم افتح لوحة التحكم.

## التثبيت

ملف تنفيذي جاهز في [Releases](https://github.com/ens-Emilio/odotoken/releases/latest)، أو من المصدر: `go build -o bin/odotoken ./cmd/odotoken`.

## التهيئة

`cp config.json.example config.json` ثم اضبط `upstream_url` لكل مزوّد.

## توجيه CLI

وجّهه إلى `http://127.0.0.1:8080` (مثل `ANTHROPIC_BASE_URL`).

## فتح لوحة التحكم

`odotoken start` ثم افتح http://127.0.0.1:8080/app/.

---

الوثائق الكاملة: [README.md](README.md).
