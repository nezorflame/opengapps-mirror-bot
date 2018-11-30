package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/google/go-github/v19/github"
	"github.com/pkg/errors"
	"github.com/pkg/profile"
	"golang.org/x/oauth2"

	"github.com/nezorflame/opengapps-mirror-bot/config"
	"github.com/nezorflame/opengapps-mirror-bot/gapps"
)

const (
	platformErrText = "does not belong to Platform values"
	androidErrText  = "does not belong to Android values"
	variantErrText  = "does not belong to Variant values"
	dateErrText     = "unable to parse time"
	mirrorCmd       = "/mirror"
	helpCmd         = "/help"
	mirrorFormat    = "[%s](%s)"
)

type tgbot struct {
	cfg *config.Config

	*tgbotapi.BotAPI
}

func main() {
	configName := ""
	flag.StringVar(&configName, "config", "config", "Config file name")
	flag.Parse()

	log.Println("Starting the bot")
	cfg, err := config.Init(configName)
	if err != nil {
		log.Fatalf("Unable to init config: %v", err)
	}
	log.Println("Config parsed")

	if cfg.EnableTracing {
		log.Println("Enabling tracing")
		defer profile.Start(profile.MemProfile, profile.CPUProfile, profile.TraceProfile).Stop()
	}

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: cfg.GithubToken},
	)
	tc := oauth2.NewClient(context.Background(), ts)
	ghClient := github.NewClient(tc)

	globalStorage := gapps.NewGlobalStorage()
	if err := globalStorage.Init(ghClient, cfg); err != nil {
		log.Fatal(err)
	}

	b, err := tgbotapi.NewBotAPI(cfg.TelegramToken)
	if err != nil {
		log.Fatal(err)
	}

	if cfg.EnableDebug {
		log.Println("Enabling debug mode")
		b.Debug = true
	}

	bot := &tgbot{cfg: cfg}
	bot.BotAPI = b
	log.Printf("Authorized on account %s", bot.Self.UserName)

	update := tgbotapi.NewUpdate(0)
	update.Timeout = bot.cfg.TelegramTimeout
	updates := bot.GetUpdatesChan(update)

	for u := range updates {
		if u.Message == nil { // ignore any non-Message Updates
			continue
		}
		switch {
		case strings.HasPrefix(u.Message.Text, mirrorCmd):
			go bot.mirror(globalStorage, ghClient, u.Message)
		case strings.HasPrefix(u.Message.Text, helpCmd):
			go bot.help(u.Message)
		}
	}
}

func (b *tgbot) mirror(gs *gapps.GlobalStorage, ghClient *github.Client, msg *tgbotapi.Message) {
	text := strings.TrimPrefix(msg.Text, mirrorCmd+" ")
	if text == "" {
		b.reply(msg.Chat.ID, msg.MessageID, b.cfg.MsgErrMirror)
		return
	}

	platform, android, variant, date, err := parseCmd(text, b.cfg.GAppsTimeFormat)
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

	s, ok := gs.Get(date)
	if !ok {
		b.reply(msg.Chat.ID, msg.MessageID, b.cfg.MsgMirrorInProgress)

		var err error
		if s, err = gapps.GetPackageStorage(ghClient, b.cfg, date); err != nil {
			b.reply(msg.Chat.ID, msg.MessageID, b.cfg.MsgErrUnknown)
			log.Fatal("No current storage available")
		}

		gs.Add(s.Date, s)
	}
	pkg, ok := s.Get(platform, android, variant)
	if !ok {
		b.reply(msg.Chat.ID, msg.MessageID, b.cfg.MsgMirrorNotFound)
		return
	}

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

	if mirrorResult == "" {
		b.reply(msg.Chat.ID, 0, b.cfg.MsgMirrorMissing)
		if err := pkg.CreateMirror(b.cfg); err != nil {
			log.Printf("Unable to create mirror: %v", err)
			b.reply(msg.Chat.ID, msg.MessageID, fmt.Sprintf(b.cfg.MsgMirrorFail, pkg.OriginURL, pkg.MD5))
			return
		}
	}
	b.reply(msg.Chat.ID, msg.MessageID, fmt.Sprintf(b.cfg.MsgMirrorOK, mirrorResult, pkg.MD5, pkg.OriginURL))
	log.Printf("Sent mirror for pkg %s: local %s, remote %s", pkg.Name, pkg.LocalURL, pkg.RemoteURL)
}

func (b *tgbot) help(msg *tgbotapi.Message) {
	b.reply(msg.Chat.ID, msg.MessageID, b.cfg.MsgHelp)
}

func (b *tgbot) reply(chatID int64, msgID int, text string) {
	msg := tgbotapi.NewMessage(chatID, fmt.Sprintf(text))
	if msgID != 0 {
		msg.ReplyToMessageID = msgID
	}
	msg.ParseMode = tgbotapi.ModeMarkdown

	if _, err := b.Send(msg); err != nil {
		log.Printf("Unable to send the message: %v", err)
		return
	}
}

func parseCmd(cmd, timeFormat string) (platform gapps.Platform, android gapps.Android, variant gapps.Variant, date string, err error) {
	cmd = strings.Replace(cmd, ".", "", -1)
	parts := strings.Split(cmd, " ")
	date = "current"
	switch len(parts) {
	case 4:
		if _, err = time.Parse(timeFormat, parts[3]); err != nil {
			err = errors.Wrap(err, dateErrText)
		}
		date = parts[3]
		fallthrough
	case 3:
		if platform, android, variant, err = gapps.ParsePackageParts(parts[:3]); err != nil {
			return
		}
	default:
		err = errors.Errorf("bad command format '%s'", cmd)
	}
	return
}
