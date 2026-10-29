# odotoken — Français

Installer odotoken, pointer votre CLI dessus, ouvrir le tableau de bord.

> Translation pending native review. The English README is authoritative.

## Installer

Binaire prêt dans [Releases](https://github.com/ens-Emilio/odotoken/releases/latest), ou depuis les sources : `go build -o bin/odotoken ./cmd/odotoken`.

## Configurer

`cp config.json.example config.json`, puis renseigner l'`upstream_url` de chaque provider.

## Pointer le CLI

Pointer vers `http://127.0.0.1:8080` (par exemple `ANTHROPIC_BASE_URL`).

## Ouvrir le tableau de bord

`odotoken start`, puis http://127.0.0.1:8080/app/.

---

Documentation complète : [README.md](README.md).
