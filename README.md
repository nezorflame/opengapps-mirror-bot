# opengapps-mirror-bot [![Go Report Card](https://goreportcard.com/badge/github.com/nezorflame/opengapps-mirror-bot)](https://goreportcard.com/report/github.com/nezorflame/opengapps-mirror-bot) [![Build Status](https://travis-ci.com/nezorflame/opengapps-mirror-bot.svg?branch=master)](https://travis-ci.com/nezorflame/opengapps-mirror-bot)

Slack bot to remind your team and team manager about the upcoming birthdays.

Requires Go 1.10+ for [go modules](https://github.com/golang/go/wiki/Modules) support.

Based on a great [tgbotapi](https://github.com/go-telegram-bot-api/telegram-bot-api) package, with the help of [go-github](https://github.com/google/go-github) package to browse Github repos.

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

### Available commands

| Command | Description |
|--------|------------------------------------------------------------|
| mirror | Searches for a OpenGApps package and creates a mirror for it |
| help | Prints the help message |

### /mirror command format

Example configuration can be found in `config.example.toml`
