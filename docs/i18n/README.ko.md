# odotoken — 한국어

odotoken을 설치하고 CLI를 연결한 뒤 대시보드를 엽니다.

> Translation pending native review. The English README is authoritative.

## 설치

[Releases](https://github.com/ens-Emilio/odotoken/releases/latest)의 빌드된 바이너리 또는 소스에서 `go build -o bin/odotoken ./cmd/odotoken`.

## 설정

`cp config.json.example config.json` 후 각 provider의 `upstream_url`을 지정합니다.

## CLI 연결

CLI를 `http://127.0.0.1:8080`으로 보냅니다 (예: `ANTHROPIC_BASE_URL`).

## 대시보드 열기

`odotoken start` 실행 후 http://127.0.0.1:8080/app/ 을 엽니다.

---

전체 문서: [README.md](README.md) (영어).
