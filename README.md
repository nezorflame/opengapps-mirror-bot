# opengapps-mirror-bot [![CircleCI](https://circleci.com/gh/nezorflame/opengapps-mirror-bot/tree/master.svg?style=svg)](https://circleci.com/gh/nezorflame/opengapps-mirror-bot/tree/master) [![Go Report Card](https://goreportcard.com/badge/github.com/nezorflame/opengapps-mirror-bot)](https://goreportcard.com/report/github.com/nezorflame/opengapps-mirror-bot) [![GolangCI](https://golangci.com/badges/github.com/nezorflame/opengapps-mirror-bot.svg)](https://golangci.com/r/github.com/nezorflame/opengapps-mirror-bot) [![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fnezorflame%2Fopengapps-mirror-bot.svg?type=shield)](https://app.fossa.com/projects/git%2Bgithub.com%2Fnezorflame%2Fopengapps-mirror-bot?ref=badge_shield)

This is a Telegram bot which allows you to create a locally and remotely hosted mirrors for any existing OpenGApps package.

Requires Go 1.10+ for [Go modules](https://github.com/golang/go/wiki/Modules) support.

Based on a great [tgbotapi](https://github.com/go-telegram-bot-api/telegram-bot-api) package with the help of [go-github](https://github.com/google/go-github) package to browse Github repos.

## Install

This project uses Go modules.
To install it, starting with Go 1.12 you can just use `go get`:

`go get opengapps-mirror-bot`

or

`go install opengapps-mirror-bot`

Also you can just clone this repo and use the build/install targets from `Makefile`.

## Usage

### Flags

| Flag | Type | Description | Default |
|--------|--------|-------------------------------------|-----------|
| config | `string` | Config file name (without extension) | `config` |

### Config

Example configuration can be found in `config.example.toml`

Local and remote mirroring can be switched on/off by entering/removing the parameters `gapps.local_url`/`gapps.remote_url` from config.

Local hosting also requires parameter `gapps.local_path`

### Available commands

| Command | Description |
|--------|------------------------------------------------------------|
| mirror | Searches for a OpenGApps package and creates a mirror for it |
| help | Prints the help message |

### /mirror command format

Targets should be put after the `/mirror` command with space character between them.

- platform: `arm`|`arm64`|`x86`|`x86_64`
- Android version: `4.4`...`9.0`
- package variant: `pico`|`nano`|`micro`|`mini`|`full`|`stock`|`super`|`aroma`|`tvstock`
- (optional) date of the release: `YYYYMMDD`

