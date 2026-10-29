# odotoken — 日本語

odotoken をインストールし、CLI を向けて、ダッシュボードを開きます。

> Translation pending native review. The English README is authoritative.

## インストール

[Releases](https://github.com/ens-Emilio/odotoken/releases/latest) のビルド済みバイナリ、またはソースから `go build -o bin/odotoken ./cmd/odotoken`。

## 設定

`cp config.json.example config.json` を実行し、各 provider の `upstream_url` を設定します。

## CLI を向ける

CLI の接続先を `http://127.0.0.1:8080` にします（例: `ANTHROPIC_BASE_URL`）。

## ダッシュボードを開く

`odotoken start` を実行し、http://127.0.0.1:8080/app/ を開きます。

---

完全なドキュメント: [README.md](README.md)（英語）。
