module github.com/nezorflame/opengapps-mirror-bot

go 1.13

require (
	github.com/go-telegram-bot-api/telegram-bot-api/v5 v5.0.0-rc1
	github.com/google/go-github/v37 v37.0.0
	github.com/nezorflame/opengapps-mirror-bot/pkg/gapps v1.3.0
	github.com/sirupsen/logrus v1.8.1
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.8.1
	go.etcd.io/bbolt v1.3.6
	golang.org/x/oauth2 v0.0.0-20210628180205-a41e5a781914
)

replace github.com/nezorflame/opengapps-mirror-bot/pkg/gapps => ./pkg/gapps
