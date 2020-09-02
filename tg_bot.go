package gocian

import (
	"time"
	"context"
	"os"
	"os/signal"
	"log"
	"io/ioutil"
	"encoding/json"

	"github.com/go-telegram-bot-api/telegram-bot-api"
	"strings"
	"fmt"
)

const TG_CONFIG_NAME = "tg_conf.json"

type TelegramBot struct {
	Config *TelegramBotConf
	API    *tgbotapi.BotAPI
}

type TelegramBotConf struct {
	Token                 string
	ReceiverIDs           []int64
	ParsesPeriodInMinutes int
}

var replacements = []string{".", "*", "_", "[", "]", "`", "-", "+", "!", "(", ")"}

func readConfiguration() (*TelegramBotConf, error) {
	conf := &TelegramBotConf{}
	file, err := ioutil.ReadFile(TG_CONFIG_NAME)
	if err == nil {
		if err = json.Unmarshal([]byte(file), conf); err == nil {
			return conf, nil
		}
	}
	data, err := json.MarshalIndent(*conf, "", "\n")
	if err != nil {
		return nil, err
	}
	if err = ioutil.WriteFile(TG_CONFIG_NAME, data, 0644); err != nil {
		return nil, err
	}
	return conf, nil
}

func (bot *TelegramBot) Initialize() {
	tgConfig, err := readConfiguration()
	if err != nil {
		panic(err)
	}
	bot.Config = tgConfig

	conf, err := ReadCianConf()
	if err != nil {
		panic(err)
	}
	parser := CianParser{Config: conf}

	api, err := tgbotapi.NewBotAPI(tgConfig.Token)
	if err != nil {
		panic(err)
	}
	bot.API = api

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, os.Kill)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		<-c
		cancel()
	}()
	ticker := time.NewTicker(time.Duration(tgConfig.ParsesPeriodInMinutes) * time.Minute)
	disabler := make(chan bool)
	disabled := make(chan bool)
	go func() {
		for {
			select {
			case <-disabler:
				disabled <- true
				return
			case <-ticker.C:
				bot.parse(&parser, &cancel)
				break
			}
		}
	}()
	log.Printf("gocian started")
	bot.parse(&parser, &cancel)
	<-ctx.Done()
	log.Printf("gocian is stopping")
	disabler <- true
	<-disabled
	log.Printf("gocian stopped")
}

func (bot *TelegramBot) parse(parser *CianParser, cancel *context.CancelFunc) {
	log.Printf("parsing new offers..")
	offers, err := parser.Parse()
	if err != nil {
		panic(err)
	}
	for _, offer := range offers {
		if err = bot.sendOffer(offer); err != nil {
			(*cancel)()
			panic(err)
			return
		}
	}
	log.Printf("sent %d offers", len(offers))
}

func (bot *TelegramBot) replaceMarkdownV2(str string) string {
	for _, repl := range replacements {
		str = strings.ReplaceAll(str, repl, "\\"+repl)
	}
	return str
}

func (bot *TelegramBot) append(key, value string) string {
	return fmt.Sprintf("*%s:* %s\\.\n", key, bot.replaceMarkdownV2(value))
}

func (bot *TelegramBot) send(message string) error {
	for _, receiverID := range bot.Config.ReceiverIDs {
		msg := tgbotapi.NewMessage(receiverID, message)
		msg.ParseMode = "MarkdownV2"
		if _, err := bot.API.Send(msg); err != nil {
			return err
		}
	}
	return nil
}

func (bot *TelegramBot) sendOffer(offer CianOffer) error {
	description := strings.ReplaceAll(offer.Description, "\n", " ")
	firstLine := fmt.Sprintf("*%s*", bot.replaceMarkdownV2(offer.GetCianUrl()))
	other := bot.append("Адрес", fmt.Sprintf("%s", offer.Address))
	other += bot.append("Цена", fmt.Sprintf("%d₽", offer.Price))
	other += bot.append("Комнат", fmt.Sprintf("%d", offer.Rooms))
	other += bot.append("Площадь", fmt.Sprintf("%.2f м², %.2f м²", offer.TotalArea, offer.LivingArea))
	other += bot.append("Этаж", fmt.Sprintf("%s", offer.FloorInfo))
	other += bot.append("Тип продажи", fmt.Sprintf("%s", offer.GetSaleType()))
	other += bot.append("Описание", fmt.Sprintf("%s", description))
	other += bot.append("Телефон для связи", fmt.Sprintf("%s", offer.Phone))

	res := fmt.Sprintf("%s\n%s\n", firstLine, other)
	return bot.send(res)
}
