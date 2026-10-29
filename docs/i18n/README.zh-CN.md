# odotoken — 简体中文

安装 odotoken，把 CLI 指向它，然后打开仪表盘。

## 安装

从 [Releases](https://github.com/ens-Emilio/odotoken/releases/latest) 下载预编译二进制，或从源码构建：`go build -o bin/odotoken ./cmd/odotoken`。

## 配置

`cp config.json.example config.json`，然后设置每个 provider 的 `upstream_url`。

## 指向 CLI

把 CLI 指向 `http://127.0.0.1:8080`（例如 `ANTHROPIC_BASE_URL`）。

## 打开仪表盘

运行 `odotoken start`，访问 http://127.0.0.1:8080/app/。

---

完整文档：[README.md](README.md)（英文）。
