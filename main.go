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

type tgbot struct {
	token   string
	timeout int

	*tgbotapi.BotAPI
}

func main() {
	log.Println("Starting the bot")
	ghClient := github.NewClient(nil)
	globalStorage := gapps.NewGlobalStorage()
	if err := globalStorage.Init(ghClient); err != nil {
		log.Fatal(err)
	}

	bot := &tgbot{token: botToken, timeout: botTimeout}
	b, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Fatal(err)
	}
	bot.BotAPI = b
	log.Printf("Authorized on account %s", bot.Self.UserName)

	update := tgbotapi.NewUpdate(0)
	update.Timeout = botTimeout
	updates := bot.GetUpdatesChan(update)

	for u := range updates {
		msg := u.Message
		if u.Message == nil { // ignore any non-Message Updates
			continue
		}
		switch {
		case strings.HasPrefix(u.Message.Text, mirrorCmd):
			bot.mirror(globalStorage, ghClient, u.Message)
		case strings.HasPrefix(msg.Text, helpCmd):
		}
	}
}

func (b *tgbot) mirror(gs *gapps.GlobalStorage, ghClient *github.Client, msg *tgbotapi.Message) {
	text := strings.TrimPrefix(msg.Text, mirrorCmd+" ")
	if text == "" {
		b.reply(msg.Chat.ID, msg.MessageID, mirrorErrMsg)
		return
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

		b.reply(msg.Chat.ID, msg.MessageID, errMsg)
		return
	}

	s, ok := gs.Get(date)
	if !ok {
		b.reply(msg.Chat.ID, msg.MessageID, inProgressMsg)

		var err error
		if s, err = gapps.GetPackageStorage(ghClient, date); err != nil {
			b.reply(msg.Chat.ID, msg.MessageID, unknownErrMsg)
			log.Fatal("No current storage available")
		}

		gs.Add(s.Date, s)
	}
	log.Printf("Got %d packages for release date %s", s.Count, s.Date)

	pkg, ok := s.Get(platform, android, variant)
	if !ok {
		b.reply(msg.Chat.ID, msg.MessageID, notFoundMsg)
		return
	}

	if pkg.MirrorURL == "" {
		b.reply(msg.Chat.ID, 0, mirrorMissingMsg)
		if err := pkg.CreateMirror(); err != nil {
			log.Printf("Unable to create mirror: %v", err)
			b.reply(msg.Chat.ID, msg.MessageID, fmt.Sprintf(mirrorFailMsg, pkg.OriginURL, pkg.MD5))
			return
		}
	}
	b.reply(msg.Chat.ID, msg.MessageID, fmt.Sprintf(mirrorMsg, pkg.MirrorURL, pkg.MD5, pkg.OriginURL))
}

func (b *tgbot) reply(chatID int64, msgID int, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	if msgID != 0 {
		msg.ReplyToMessageID = msgID
	}

	if _, err := b.Send(msg); err != nil {
		log.Printf("Unable to send the message: %v", err)
		return
	}
	log.Println(text)
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
