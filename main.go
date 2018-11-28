package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/google/go-github/v19/github"
	"github.com/pkg/errors"

	"github.com/nezorflame/opengapps-mirror-bot/gapps"
)

var (
	botToken   = "someToken"
	botTimeout = 60
)

func main() {
	log.Println("Starting the bot")
	ghClient := github.NewClient(nil)
	globalStorage := gapps.NewGlobalStorage()
	if err := globalStorage.Init(ghClient); err != nil {
		log.Fatal(err)
	}

	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Fatal(err)
	}

	// bot.Debug = true
	log.Printf("Authorized on account %s", bot.Self.UserName)

	update := tgbotapi.NewUpdate(0)
	update.Timeout = botTimeout
	updates := bot.GetUpdatesChan(update)

	for u := range updates {
		if u.Message == nil || !strings.HasPrefix(u.Message.Text, mirrorCmd) { // ignore any non-Message Updates
			continue
		}
		log.Printf("[%s] %s", u.Message.From.UserName, u.Message.Text)

		text := strings.TrimPrefix(u.Message.Text, mirrorCmd+" ")
		if text == "" {
			sendReply(bot, u.Message.Chat.ID, u.Message.MessageID, mirrorErrMsg)
			continue
		}

		platform, android, variant, date, err := parseCmd(text)
		if err != nil {
			errMsg := err.Error()
			switch {
			case strings.Contains(errMsg, platformErrText):
				errMsg = platformErrMsg
			case strings.Contains(errMsg, androidErrText):
				errMsg = androidErrMsg
			case strings.Contains(errMsg, variantErrText):
				errMsg = variantErrMsg
			case strings.Contains(errMsg, dateErrText):
				errMsg = dateErrMsg
			default:
				errMsg = unknownErrMsg
			}

			sendReply(bot, u.Message.Chat.ID, u.Message.MessageID, errMsg)
			continue
		}

		s, ok := globalStorage.Get(date)
		if !ok {
			sendReply(bot, u.Message.Chat.ID, u.Message.MessageID, inProgressMsg)

			var err error
			if s, err = gapps.GetPackageStorage(ghClient, date); err != nil {
				sendReply(bot, u.Message.Chat.ID, u.Message.MessageID, unknownErrMsg)
				log.Fatal("No current storage available")
			}

			globalStorage.Add(s.Date, s)
		}
		log.Printf("Got %d packages for release date %s", s.Count, s.Date)

		pkg, ok := s.Get(platform, android, variant)
		if !ok {
			sendReply(bot, u.Message.Chat.ID, u.Message.MessageID, notFoundMsg)
			continue
		}

		sendReply(bot, u.Message.Chat.ID, 0, fmt.Sprintf(foundMsg, pkg.Name))

		if pkg.MirrorURL == "" {
			sendReply(bot, u.Message.Chat.ID, 0, mirrorMissingMsg)
			if err := pkg.CreateMirror(); err != nil {
				sendReply(bot, u.Message.Chat.ID, u.Message.MessageID, fmt.Sprintf(mirrorFailMsg, pkg.OriginURL, pkg.MD5))
				continue
			}
		}
		sendReply(bot, u.Message.Chat.ID, u.Message.MessageID, fmt.Sprintf(mirrorMsg, pkg.MirrorURL, pkg.MD5, pkg.OriginURL))
	}
}

func parseCmd(cmd string) (platform gapps.Platform, android gapps.Android, variant gapps.Variant, date string, err error) {
	cmd = strings.Replace(cmd, ".", "", -1)
	parts := strings.Split(cmd, " ")
	date = "current"
	switch len(parts) {
	case 4:
		if _, err = time.Parse(gapps.TimeFormat, parts[3]); err != nil {
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

func sendReply(bot *tgbotapi.BotAPI, chatID int64, msgID int, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	if msgID != 0 {
		msg.ReplyToMessageID = msgID
	}

	if _, err := bot.Send(msg); err != nil {
		log.Printf("Unable to send the message: %v", err)
		return
	}
	log.Println(text)
}
