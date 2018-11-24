package main

import (
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/google/go-github/v19/github"

	"github.com/nezorflame/opengapps-mirror-bot/gapps"
)

var (
	botToken   = "someToken"
	botTimeout = 60
	mirrorCmd  = "/mirror"
)

func main() {
	ghClient := github.NewClient(nil)
	packageStorage, err := gapps.GetPackageStorage(ghClient)
	if err != nil {
		log.Fatalf("Unable to get packages: %v", err)
	}

	log.Printf("Got %d packages for release date %s", packageStorage.Count, packageStorage.Date)

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
		if u.Message == nil || u.Message.Text != mirrorCmd { // ignore any non-Message Updates
			continue
		}

		log.Printf("[%s] %s", u.Message.From.UserName, u.Message.Text)

		msg := tgbotapi.NewMessage(u.Message.Chat.ID, u.Message.Text)
		msg.ReplyToMessageID = u.Message.MessageID

		if _, err := bot.Send(msg); err != nil {
			log.Printf("Unable to send the message: %v", err)
		}
	}
}
