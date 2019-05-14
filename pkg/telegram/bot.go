package telegram

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/nezorflame/opengapps-mirror-bot/internal/pkg/storage"
	"github.com/nezorflame/opengapps-mirror-bot/pkg/gapps"
	"github.com/nezorflame/opengapps-mirror-bot/pkg/net"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/google/go-github/v25/github"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

const (
	platformErrText = "does not belong to Platform values"
	androidErrText  = "does not belong to Android values"
	variantErrText  = "does not belong to Variant values"
	dateErrText     = "unable to parse time"
	mirrorFormat    = "[%s](%s)"
)

// Bot describes Telegram bot
type Bot struct {
	ctx context.Context
	api *tgbotapi.BotAPI
	cfg *viper.Viper
	dq  *net.DownloadQueue
	gs  *storage.GlobalStorage
	gh  *github.Client
}

// NewBot creates new instance of Bot
func NewBot(ctx context.Context, cfg *viper.Viper, dq *net.DownloadQueue, gs *storage.GlobalStorage, gh *github.Client) (*Bot, error) {
	if cfg == nil {
		return nil, errors.New("empty config")
	}

	api, err := tgbotapi.NewBotAPI(cfg.GetString("telegram.token"))
	if err != nil {
		return nil, errors.Wrap(err, "unable to connect to Telegram")
	}
	if cfg.GetBool("telegram.debug") {
		log.Debug("Enabling debug mode for bot")
		api.Debug = true
	}

	log.Debugf("Authorized on account %s", api.Self.UserName)
	return &Bot{api: api, cfg: cfg, ctx: ctx, dq: dq, gs: gs, gh: gh}, nil
}

// Start starts to listen the bot updates channel
func (b *Bot) Start() {
	update := tgbotapi.NewUpdate(0)
	update.Timeout = b.cfg.GetInt("telegram.timeout")
	b.listen(b.api.GetUpdatesChan(update))
}

// Stop stops the bot
func (b *Bot) Stop() {
	b.api.StopReceivingUpdates()
}

func (b *Bot) listen(updates tgbotapi.UpdatesChannel) {
	for u := range updates {
		if u.Message == nil { // ignore any non-Message Updates
			continue
		}

		switch {
		case strings.HasPrefix(u.Message.Text, b.cfg.GetString("commands.start")):
			go b.hello(u.Message)
		case strings.HasPrefix(u.Message.Text, b.cfg.GetString("commands.help")):
			log.WithField("user_id", u.Message.From.ID).Debug("Got help request")
			go b.help(u.Message)
		case strings.HasPrefix(u.Message.Text, b.cfg.GetString("commands.mirror")):
			log.WithField("user_id", u.Message.From.ID).Debug("Got mirror request")
			go b.mirror(u.Message)
		}
	}
}

func (b *Bot) hello(msg *tgbotapi.Message) {
	b.reply(msg.Chat.ID, msg.MessageID, b.cfg.GetString("messages.hello"))
}

func (b *Bot) help(msg *tgbotapi.Message) {
	b.reply(msg.Chat.ID, msg.MessageID, b.cfg.GetString("messages.help"))
}

func (b *Bot) mirror(msg *tgbotapi.Message) {
	// parse the message
	logger := log.WithField("chat_id", msg.Chat.ID).WithField("msg_id", msg.MessageID)
	cmd := strings.Replace(msg.Text, ".", "", -1)
	parts := strings.Split(cmd, " ")
	if len(parts) < 2 {
		b.reply(msg.Chat.ID, msg.MessageID, b.cfg.GetString("messages.errors.mirror"))
		return
	}

	platform, android, variant, date, err := parseCmd(parts[1:], b.cfg.GetString("gapps.time_format"))
	if err != nil {
		errMsg := err.Error()
		switch {
		case strings.Contains(errMsg, platformErrText):
			errMsg = b.cfg.GetString("messages.errors.platform")
		case strings.Contains(errMsg, androidErrText):
			errMsg = b.cfg.GetString("messages.errors.android")
		case strings.Contains(errMsg, variantErrText):
			errMsg = b.cfg.GetString("messages.errors.variant")
		case strings.Contains(errMsg, dateErrText):
			errMsg = b.cfg.GetString("messages.errors.date")
		default:
			errMsg = b.cfg.GetString("messages.errors.mirror")
		}

		b.reply(msg.Chat.ID, msg.MessageID, errMsg)
		return
	}

	// look up the package storage
	s, ok := b.gs.Get(date)
	if !ok {
		b.reply(msg.Chat.ID, msg.MessageID, b.cfg.GetString("messages.mirror.in_progress"))

		if s, err = storage.GetPackageStorage(b.ctx, b.gh, b.dq, b.cfg, date); err != nil {
			b.reply(msg.Chat.ID, msg.MessageID, b.cfg.GetString("messages.errors.unknown"))
			logger.Fatal("No current storage available")
		}

		b.gs.Add(s.Date, s)
	}

	// look up the package
	pkg, ok := s.Get(platform, android, variant)
	if !ok {
		// try to reform the package
		var st *storage.Storage
		if st, err = storage.GetPackageStorage(b.ctx, b.gh, b.dq, b.cfg, date); err == nil {
			s, ok = storage.MergeStoragePackages(s, st)
			if ok {
				if err = s.Save(); err == nil {
					pkg, ok = s.Get(platform, android, variant)
					if !ok {
						err = errors.New("package is missing from the updated storage")
					}
				}
			} else {
				err = errors.New("package is missing from the storage despite the update")
			}
		}

		// report error and exit
		if err != nil {
			logger.WithError(err).Error("Unable to get the package")
			b.reply(msg.Chat.ID, msg.MessageID, b.cfg.GetString("messages.mirror.not_found"))
			return
		}
	}

	// check if we already have mirrors
	text := ""
	if pkg.LocalURL == "" && pkg.RemoteURL == "" {
		text = fmt.Sprintf(b.cfg.GetString("messages.mirror.found"), pkg.Name, pkg.OriginURL, pkg.MD5, b.cfg.GetString("messages.mirror.missing"))
		b.reply(msg.Chat.ID, 0, text)
		logger.Debugf("Creating a mirror for the package %s", pkg.Name)
		if err := pkg.CreateMirror(b.dq, b.cfg); err != nil {
			logger.Errorf("Unable to create mirror: %v", err)
			b.reply(msg.Chat.ID, msg.MessageID, b.cfg.GetString("messages.mirror.fail"))
			return
		}
		if err := s.Save(); err != nil {
			logger.Errorf("Unable to save storage: %v", err)
		}
		text = b.cfg.GetString("messages.mirror.ok")
	} else {
		text = fmt.Sprintf(b.cfg.GetString("messages.mirror.found"), pkg.Name, pkg.OriginURL, pkg.MD5, b.cfg.GetString("messages.mirror.ok"))
	}

	logger.Debugf("Got the mirror for the package %s", pkg.Name)
	mirrorResult := ""
	if pkg.LocalURL != "" {
		mirrorResult = fmt.Sprintf(mirrorFormat, b.cfg.GetString("gapps.local_host"), pkg.LocalURL)
	}
	if pkg.RemoteURL != "" {
		if mirrorResult != "" {
			mirrorResult += " | "
		}
		mirrorResult += fmt.Sprintf(mirrorFormat, b.cfg.GetString("gapps.remote_host"), pkg.RemoteURL)
	}

	b.reply(msg.Chat.ID, msg.MessageID, fmt.Sprintf(text, mirrorResult))
	logger.Infof("Sent mirror for pkg %s", pkg.Name)
}

func (b *Bot) reply(chatID int64, msgID int, text string) {
	log.WithField("chat_id", chatID).WithField("msg_id", msgID).Debug("Sending reply")
	msg := tgbotapi.NewMessage(chatID, fmt.Sprint(text))
	if msgID != 0 {
		msg.ReplyToMessageID = msgID
	}
	msg.ParseMode = tgbotapi.ModeMarkdown

	if _, err := b.api.Send(msg); err != nil {
		log.Errorf("Unable to send the message: %v", err)
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
