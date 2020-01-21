package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nezorflame/opengapps-mirror-bot/internal/pkg/config"
	"github.com/nezorflame/opengapps-mirror-bot/internal/pkg/db"
	"github.com/nezorflame/opengapps-mirror-bot/internal/pkg/storage"
	"github.com/nezorflame/opengapps-mirror-bot/pkg/net"
	"github.com/nezorflame/opengapps-mirror-bot/pkg/telegram"

	"github.com/google/go-github/v29/github"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"golang.org/x/oauth2"
)

var configName string

func init() {
	// get flags, init logger
	pflag.StringVar(&configName, "config", "config", "Config file name")
	level := pflag.String("log-level", "INFO", "Logrus log level (DEBUG, WARN, etc.)")
	pflag.Parse()

	logLevel, err := log.ParseLevel(*level)
	if err != nil {
		log.Fatalf("Unknown log level: %s", *level)
	}
	log.SetFormatter(&log.TextFormatter{FullTimestamp: true})
	log.SetOutput(os.Stdout)
	log.SetLevel(logLevel)

	if configName == "" {
		pflag.PrintDefaults()
		os.Exit(1)
	}
}

func main() {
	// init flags and ctx
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// init config and tracing
	log.Info("Starting the bot")
	cfg, err := config.New(configName)
	if err != nil {
		log.Fatalf("Unable to init config: %v", err)
	}
	log.Info("Config parsed")

	// init Github client
	log.Info("Creating Github client")
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: cfg.GetString("github.token")},
	)
	tc := oauth2.NewClient(ctx, ts)
	gh := github.NewClient(tc)

	// init download queue and cache
	log.Info("Creating download queue")
	dq := net.NewQueue(cfg.GetInt("max_downloads"))
	cache, err := db.NewDB(cfg.GetString("db.path"), cfg.GetDuration("db.timeout"))
	if err != nil {
		log.Fatal(err)
	}

	// init GApps global storage
	log.Info("Initiating GApps global storage")
	gs := storage.NewGlobalStorage(cache)
	if err = gs.Load(); err != nil {
		log.Fatalf("Unable to load the global storage from cache: %v", err)
	}

	if err = gs.AddLatestStorage(ctx, gh, dq, cfg); err != nil {
		log.Fatalf("Unable to add the latest storage: %v", err)
	}

	// init package watcher
	log.Info("Initiating GApps package watcher")
	go func() {
		ticker := time.NewTicker(cfg.GetDuration("gapps.renew_period"))
		for {
			select {
			case <-ticker.C:
				log.Info("Updating the current storage")
				if err = gs.AddLatestStorage(ctx, gh, dq, cfg); err != nil {
					log.Errorf("Unable to add the latest storage: %v", err)
				}
			case <-ctx.Done():
				log.Warnf("Closing the watcher by context: %v", ctx.Err())
				ticker.Stop()
				return
			}
		}
	}()

	// create bot
	bot, err := telegram.NewBot(ctx, cfg, dq, gs, gh)
	if err != nil {
		log.WithError(err).Fatal("Unable to create bot")
	}
	log.Info("Bot created")

	// init graceful stop chan
	log.Debug("Initiating system signal watcher")
	var gracefulStop = make(chan os.Signal)
	signal.Notify(gracefulStop, syscall.SIGTERM)
	signal.Notify(gracefulStop, syscall.SIGINT)

	go func() {
		sig := <-gracefulStop
		log.Warnf("Caught sig %+v, stopping the app", sig)
		cancel()
		bot.Stop()
		gs.Save()
		if err = cache.Close(false); err != nil {
			log.WithError(err).Error("Unable to close DB")
		}
		os.Exit(0)
	}()

	// start the bot
	log.Info("Starting the bot")
	bot.Start()
}
