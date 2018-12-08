package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/google/go-github/v19/github"
	"github.com/pkg/errors"
	"github.com/pkg/profile"
	"go.uber.org/zap"
	"golang.org/x/oauth2"

	"github.com/nezorflame/opengapps-mirror-bot/lib/config"
	"github.com/nezorflame/opengapps-mirror-bot/lib/db"
	"github.com/nezorflame/opengapps-mirror-bot/lib/gapps"
	"github.com/nezorflame/opengapps-mirror-bot/lib/utils"
)

const (
	platformErrText = "does not belong to Platform values"
	androidErrText  = "does not belong to Android values"
	variantErrText  = "does not belong to Variant values"
	dateErrText     = "unable to parse time"
	startCmd        = "/start"
	mirrorCmd       = "/mirror"
	helpCmd         = "/help"
	mirrorFormat    = "[%s](%s)"
)

type tgbot struct {
	cfg *config.Config
	dq  *utils.DownloadQueue
	log *zap.SugaredLogger
	*tgbotapi.BotAPI
}

func main() {
	// init flags
	configName := flag.String("config", "config", "Config file name")
	level := zap.LevelFlag("log", zap.InfoLevel, "Log level")
	flag.Parse()

	// init logger
	logConfig := zap.NewProductionConfig()
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
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: cfg.GithubToken},
	)
	tc := oauth2.NewClient(context.Background(), ts)
	ghClient := github.NewClient(tc)

	// init download queue and cache
	dq := utils.NewQueue(log, cfg.MaxDownloads)
	cache, err := db.NewDB(log, cfg.DBPath, cfg.DBTimeout)
	if err != nil {
		log.Fatal(err)
	}

	// init GApps storage
	globalStorage := gapps.NewGlobalStorage(log, cache)
	if err = globalStorage.Init(ghClient, dq, cfg); err != nil {
		log.Fatal(err)
	}

	// init graceful stop chan
	var gracefulStop = make(chan os.Signal)
	signal.Notify(gracefulStop, syscall.SIGTERM)
	signal.Notify(gracefulStop, syscall.SIGINT)
	go func() {
		sig := <-gracefulStop
		log.Warnf("Caught sig %+v, stopping the app", sig)
		globalStorage.Save()
		if err = cache.Close(false); err != nil {
			log.Errorf("Unable to close DB: %v", err)
		}
		log.Sync()
		os.Exit(0)
	}()

	// init bot
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
	update := tgbotapi.NewUpdate(0)
	update.Timeout = bot.cfg.TelegramTimeout
	updates := bot.GetUpdatesChan(update)
	for u := range updates {
		if u.Message == nil { // ignore any non-Message Updates
			continue
		}
		switch {
		case strings.HasPrefix(u.Message.Text, startCmd):
			go bot.hello(u.Message)
		case strings.HasPrefix(u.Message.Text, helpCmd):
			go bot.help(u.Message)
		case strings.HasPrefix(u.Message.Text, mirrorCmd):
			go bot.mirror(globalStorage, ghClient, u.Message)
		}
	}
}

func (b *tgbot) hello(msg *tgbotapi.Message) {
	b.reply(msg.Chat.ID, msg.MessageID, b.cfg.MsgHello)
}

func (b *tgbot) help(msg *tgbotapi.Message) {
	b.reply(msg.Chat.ID, msg.MessageID, b.cfg.MsgHelp)
}

func (b *tgbot) mirror(gs *gapps.GlobalStorage, ghClient *github.Client, msg *tgbotapi.Message) {
	// parse the message
	cmd := strings.Replace(msg.Text, ".", "", -1)
	parts := strings.Split(cmd, " ")
	if len(parts) < 2 {
		b.reply(msg.Chat.ID, msg.MessageID, b.cfg.MsgErrMirror)
		return
	}

	platform, android, variant, date, err := parseCmd(parts[1:], b.cfg.GAppsTimeFormat)
	if err != nil {
		errMsg := err.Error()
		switch {
		case strings.Contains(errMsg, platformErrText):
			errMsg = b.cfg.MsgErrPlatform
		case strings.Contains(errMsg, androidErrText):
			errMsg = b.cfg.MsgErrAndroid
		case strings.Contains(errMsg, variantErrText):
			errMsg = b.cfg.MsgErrVariant
		case strings.Contains(errMsg, dateErrText):
			errMsg = b.cfg.MsgErrDate
		default:
			errMsg = b.cfg.MsgErrMirror
		}

		b.reply(msg.Chat.ID, msg.MessageID, errMsg)
		return
	}

	// look up the package storage
	s, ok := gs.Get(date)
	if !ok {
		b.reply(msg.Chat.ID, msg.MessageID, b.cfg.MsgMirrorInProgress)

		var err error
		if s, err = gapps.GetPackageStorage(b.log, ghClient, b.dq, b.cfg, date); err != nil {
			b.reply(msg.Chat.ID, msg.MessageID, b.cfg.MsgErrUnknown)
			b.log.Fatal("No current storage available")
		}

		gs.Add(s.Date, s)
	}

	// look up the package
	pkg, ok := s.Get(platform, android, variant)
	if !ok {
		b.reply(msg.Chat.ID, msg.MessageID, b.cfg.MsgMirrorNotFound)
		return
	}

	// check if we already have mirrors
	text := ""
	if pkg.LocalURL == "" && pkg.RemoteURL == "" {
		text = fmt.Sprintf(b.cfg.MsgMirrorFound, pkg.Name, pkg.OriginURL, pkg.MD5, b.cfg.MsgMirrorMissing)
		b.reply(msg.Chat.ID, 0, text)
		b.log.Debugf("Creating a mirror for the package %s", pkg.Name)
		if err := pkg.CreateMirror(b.log, b.dq, b.cfg); err != nil {
			b.log.Errorf("Unable to create mirror: %v", err)
			b.reply(msg.Chat.ID, msg.MessageID, b.cfg.MsgMirrorFail)
			return
		}
		if err := s.Save(); err != nil {
			b.log.Errorf("Unable to save storage: %v", err)
		}
		text = b.cfg.MsgMirrorOK
	} else {
		text = fmt.Sprintf(b.cfg.MsgMirrorFound, pkg.Name, pkg.OriginURL, pkg.MD5, b.cfg.MsgMirrorOK)
	}

	b.log.Debugf("Got the mirror for the package %s", pkg.Name)
	mirrorResult := ""
	if pkg.LocalURL != "" {
		mirrorResult = fmt.Sprintf(mirrorFormat, b.cfg.GAppsLocalHostname, pkg.LocalURL)
	}
	if pkg.RemoteURL != "" {
		if mirrorResult != "" {
			mirrorResult += " | "
		}
		mirrorResult += fmt.Sprintf(mirrorFormat, b.cfg.GAppsRemoteHostname, pkg.RemoteURL)
	}

	b.reply(msg.Chat.ID, msg.MessageID, fmt.Sprintf(text, mirrorResult))
	b.log.Infof("Sent mirror for pkg %s", pkg.Name)
}

func (b *tgbot) reply(chatID int64, msgID int, text string) {
	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf(text))
	if msgID != 0 {
		msg.ReplyToMessageID = msgID
	}
	msg.ParseMode = tgbotapi.ModeMarkdown

	if _, err := b.Send(msg); err != nil {
		b.log.Errorf("Unable to send the message: %v", err)
		return
	}
}

func parseCmd(parts []string, timeFormat string) (platform gapps.Platform, android gapps.Android, variant gapps.Variant, date string, err error) {
	date = "current"
	switch len(parts) {
	case 4:
		if _, err = time.Parse(timeFormat, parts[3]); err != nil {
			err = errors.Wrap(err, dateErrText)
			return
		}
		date = parts[3]
		fallthrough
	case 3:
		if platform, android, variant, err = gapps.ParsePackageParts(parts[:3]); err != nil {
			return
		}
	default:
		err = errors.New("bad command format")
	}
	return
}
