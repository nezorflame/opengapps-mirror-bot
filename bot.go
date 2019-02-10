package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/google/go-github/v19/github"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/nezorflame/opengapps-mirror-bot/lib/config"
	"github.com/nezorflame/opengapps-mirror-bot/lib/gapps"
	"github.com/nezorflame/opengapps-mirror-bot/lib/utils"
)

type tgbot struct {
	cfg *config.Config
	dq  *utils.DownloadQueue
	log *zap.SugaredLogger
	*tgbotapi.BotAPI
}

func (b *tgbot) hello(msg *tgbotapi.Message) {
	b.reply(msg.Chat.ID, msg.MessageID, b.cfg.MsgHello)
}

func (b *tgbot) help(msg *tgbotapi.Message) {
	b.reply(msg.Chat.ID, msg.MessageID, b.cfg.MsgHelp)
}

func (b *tgbot) mirror(ctx context.Context, gs *gapps.GlobalStorage, ghClient *github.Client, msg *tgbotapi.Message) {
	// parse the message
	logger := b.log.With("chat_id", msg.Chat.ID, "msg_id", msg.MessageID)
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
		if s, err = gapps.GetPackageStorage(ctx, logger, ghClient, b.dq, b.cfg, date); err != nil {
			b.reply(msg.Chat.ID, msg.MessageID, b.cfg.MsgErrUnknown)
			logger.Fatal("No current storage available")
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
		logger.Debugf("Creating a mirror for the package %s", pkg.Name)
		if err := pkg.CreateMirror(logger, b.dq, b.cfg); err != nil {
			logger.Errorf("Unable to create mirror: %v", err)
			b.reply(msg.Chat.ID, msg.MessageID, b.cfg.MsgMirrorFail)
			return
		}
		if err := s.Save(); err != nil {
			logger.Errorf("Unable to save storage: %v", err)
		}
		text = b.cfg.MsgMirrorOK
	} else {
		text = fmt.Sprintf(b.cfg.MsgMirrorFound, pkg.Name, pkg.OriginURL, pkg.MD5, b.cfg.MsgMirrorOK)
	}

	logger.Debugf("Got the mirror for the package %s", pkg.Name)
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
	logger.Infof("Sent mirror for pkg %s", pkg.Name)
}

func (b *tgbot) reply(chatID int64, msgID int, text string) {
	b.log.With("chat_id", chatID, "msg_id", msgID).Debug("Sending reply")
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
