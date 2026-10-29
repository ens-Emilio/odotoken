# odotoken — हिन्दी

odotoken इंस्टॉल करें, अपने CLI को उस पर इंगित करें और डैशबोर्ड खोलें।

> Translation pending native review. The English README is authoritative.

## इंस्टॉल

[Releases](https://github.com/ens-Emilio/odotoken/releases/latest) से तैयार बाइनरी, या स्रोत से: `go build -o bin/odotoken ./cmd/odotoken`।

## कॉन्फ़िगर

`cp config.json.example config.json`, फिर हर provider का `upstream_url` सेट करें।

## CLI इंगित करें

`http://127.0.0.1:8080` पर भेजें (जैसे `ANTHROPIC_BASE_URL`)।

## डैशबोर्ड खोलें

`odotoken start`, फिर http://127.0.0.1:8080/app/।

---

पूरा दस्तावेज़: [README.md](README.md)।
