# opengapps-mirror-bot [![CircleCI](https://circleci.com/gh/nezorflame/opengapps-mirror-bot/tree/master.svg?style=svg)](https://circleci.com/gh/nezorflame/opengapps-mirror-bot/tree/master) [![Go Report Card](https://goreportcard.com/badge/github.com/nezorflame/opengapps-mirror-bot)](https://goreportcard.com/report/github.com/nezorflame/opengapps-mirror-bot) [![GolangCI](https://golangci.com/badges/github.com/nezorflame/opengapps-mirror-bot.svg)](https://golangci.com/r/github.com/nezorflame/opengapps-mirror-bot)

This is a Telegram bot which allows you to create a locally and remotely hosted mirrors for any existing OpenGApps package.

Requires Go 1.10+ for [go modules](https://github.com/golang/go/wiki/Modules) support.

Based on a great [tgbotapi](https://github.com/go-telegram-bot-api/telegram-bot-api) package with the help of [go-github](https://github.com/google/go-github) package to browse Github repos.

## Install

1. Get the bot:
    ```bash
    go get github.com/nezorflame/opengapps-mirror-bot
    cd $GOPATH/src/github.com/nezorflame/opengapps-mirror-bot
    ```
2. Install the dependencies and the bot itself:
    - with `vgo`:
    ```bash
    export GO111MODULE=on
    go install
    ```

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

