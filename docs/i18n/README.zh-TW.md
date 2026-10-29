# odotoken — 繁體中文

安裝 odotoken，把 CLI 指向它，然後開啟儀表板。

> Translation pending native review. The English README is authoritative.

## 安裝

從 [Releases](https://github.com/ens-Emilio/odotoken/releases/latest) 取得預先編譯的執行檔，或從原始碼建置：`go build -o bin/odotoken ./cmd/odotoken`。

## 設定

`cp config.json.example config.json`，然後設定每個 provider 的 `upstream_url`。

## 指向 CLI

把 CLI 指向 `http://127.0.0.1:8080`（例如 `ANTHROPIC_BASE_URL`）。

## 開啟儀表板

執行 `odotoken start`，開啟 http://127.0.0.1:8080/app/。

---

完整文件：[README.md](README.md)（英文）。
