package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/pkg/profile"
	"go.uber.org/zap"

	"github.com/nezorflame/opengapps-mirror-bot/lib/config"
	"github.com/nezorflame/opengapps-mirror-bot/lib/db"
	"github.com/nezorflame/opengapps-mirror-bot/lib/utils"
)

const (
	platformErrText = "does not belong to Platform values"
	androidErrText  = "does not belong to Android values"
	variantErrText  = "does not belong to Variant values"
	dateErrText     = "unable to parse time"
	mirrorFormat    = "[%s](%s)"
)

func main() {
	// init flags and ctx
	// ctx, cancel := context.WithCancel(context.Background())
	// defer cancel()
	configName := flag.String("config", "config", "Config file name")
	level := zap.LevelFlag("log", zap.InfoLevel, "Log level")
	flag.Parse()

	// init logger
	logConfig := zap.NewDevelopmentConfig()
	logConfig.Encoding = "json"
	logConfig.Level.SetLevel(*level)
	logger, err := logConfig.Build()
	if err != nil {
		panic(err)
	}
	log := logger.Sugar()

	// init config and tracing
	log.Info("Starting the bot")
	cfg, err := config.Init(*configName)
	if err != nil {
		log.Fatalf("Unable to init config: %v", err)
	}
	log.Info("Config parsed")
	if cfg.EnableTracing {
		log.Debug("Enabling tracing")
		defer profile.Start(profile.MemProfile, profile.CPUProfile, profile.TraceProfile).Stop()
	}

	// init Github client
	// log.Info("Creating Github client")
	// ts := oauth2.StaticTokenSource(
	// 	&oauth2.Token{AccessToken: cfg.GithubToken},
	// )
	// tc := oauth2.NewClient(ctx, ts)
	// ghClient := github.NewClient(tc)

	// init download queue and cache
	log.Info("Creating download queue")
	dq := utils.NewQueue(log, cfg.MaxDownloads)
	cache, err := db.NewDB(log, cfg.DBPath, cfg.DBTimeout)
	if err != nil {
		log.Fatal(err)
	}

	// init GApps global storage
	// log.Info("Initiating GApps global storage")
	// globalStorage := gapps.NewGlobalStorage(log, cache)
	// if err = globalStorage.Load(); err != nil {
	// 	log.Fatalf("Unable to load the global storage from cache: %v", err)
	// }

	// if err = globalStorage.AddLatest(ctx, ghClient, dq, cfg); err != nil {
	// 	log.Fatalf("Unable to add the latest storage: %v", err)
	// }

	// init package watcher
	// log.Info("Initiating GApps package watcher")
	// go func(ctx context.Context) {
	// 	ticker := time.NewTicker(cfg.GAppsRenewPeriod)
	// 	for {
	// 		select {
	// 		case <-ticker.C:
	// 			log.Info("Updating the current storage")
	// 			if err = globalStorage.AddLatest(ctx, ghClient, dq, cfg); err != nil {
	// 				log.Errorf("Unable to add the latest storage: %v", err)
	// 			}
	// 		case <-ctx.Done():
	// 			log.Warnf("Closing the watcher by context: %v", ctx.Err())
	// 			ticker.Stop()
	// 			return
	// 		}
	// 	}
	// }(ctx)

	// init graceful stop chan
	log.Info("Initiating system signal watcher")
	var gracefulStop = make(chan os.Signal)
	signal.Notify(gracefulStop, syscall.SIGTERM)
	signal.Notify(gracefulStop, syscall.SIGINT)
	go func() {
		sig := <-gracefulStop
		log.Warnf("Caught sig %+v, stopping the app", sig)
		// cancel()
		// globalStorage.Save()
		if err = cache.Close(false); err != nil {
			log.Errorf("Unable to close DB: %v", err)
		}
		_ = log.Sync()
		os.Exit(0)
	}()

	// init bot
	log.Info("Creating Telegram bot")
	b, err := tgbotapi.NewBotAPI(cfg.TelegramToken)
	if err != nil {
		log.Fatal(err)
	}
	if cfg.EnableTracing {
		log.Debug("Enabling debug mode for bot")
		b.Debug = true
	}
	bot := &tgbot{
		BotAPI: b,
		log:    log,
		cfg:    cfg,
		dq:     dq,
	}
	log.Debugf("Authorized on account %s", bot.Self.UserName)

	// start listening to the updates
	log.Info("Bot created. Listening to the updates")
	update := tgbotapi.NewUpdate(0)
	update.Timeout = bot.cfg.TelegramTimeout
	updates := bot.GetUpdatesChan(update)
	for u := range updates {
		if u.Message == nil { // ignore any non-Message Updates
			continue
		}
		switch {
		default:
			go bot.hello(u.Message)
			// case strings.HasPrefix(u.Message.Text, helpCmd):
			// 	log.With("user_id", u.Message.From.ID).Debug("Got /help request")
			// 	go bot.help(u.Message)
			// case strings.HasPrefix(u.Message.Text, mirrorCmd):
			// 	log.With("user_id", u.Message.From.ID).Debug("Got /mirror request")
			// 	go bot.mirror(ctx, globalStorage, ghClient, u.Message)
		}
	}
}
