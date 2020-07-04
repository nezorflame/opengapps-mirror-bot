module github.com/nezorflame/opengapps-mirror-bot

go 1.13

require (
	github.com/go-telegram-bot-api/telegram-bot-api/v5 v5.0.0-rc1
	github.com/google/go-github/v32 v32.0.0
	github.com/nezorflame/opengapps-mirror-bot/pkg/gapps v1.2.0
	github.com/sirupsen/logrus v1.6.0
	github.com/spf13/pflag v1.0.5
	github.com/spf13/viper v1.7.0
	go.etcd.io/bbolt v1.3.5
	golang.org/x/oauth2 v0.0.0-20200107190931-bf48bf16ab8d
)

replace github.com/nezorflame/opengapps-mirror-bot/pkg/gapps => ./pkg/gapps
