package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
)

type Weather struct {
	City      string  `json:"city"`
	Temp      float64 `json:"temp_celsius"`
	FeelsLike float64 `json:"feels_like"`
	Humidity  int     `json:"humidity"`
	Condition string  `json:"condition"`
}

func main() {
	// Загружаем .env локально, если нужно
	if os.Getenv("RENDER") == "" {
		_ = godotenv.Load()
	}

	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		log.Fatal("❌ TELEGRAM_BOT_TOKEN не задан")
	}

	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Fatal("❌ Ошибка инициализации бота:", err)
	}
	bot.Debug = true
	log.Printf("✅ Авторизован как @%s", bot.Self.UserName)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("✅ OK\nBot: @" + bot.Self.UserName))
	})

	go func() {
		log.Printf("📡 HTTP сервер слушает :%s", port)
		if err := http.ListenAndServe(":"+port, nil); err != nil {
			log.Fatal(err)
		}
	}()

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)

	log.Println("🚀 Бот запущен и ожидает сообщений")

	for update := range updates {
		if update.Message == nil {
			continue
		}

		log.Printf("[%s] %s", update.Message.From.UserName, update.Message.Text)

		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")

		if update.Message.IsCommand() {
			handleCommand(update, &msg)
		} else {
			handleTextMessage(update, &msg)
		}

		if _, err := bot.Send(msg); err != nil {
			log.Printf("⚠️ Ошибка отправки сообщения: %v", err)
		}
	}
}

func handleCommand(update tgbotapi.Update, msg *tgbotapi.MessageConfig) {
	command := update.Message.Command()
	switch command {
	case "start":
		msg.Text = "Привет! Я погодный бот.\nИспользуй:\n/weather <город> — узнать погоду"
	case "weather":
		args := update.Message.CommandArguments()
		if args == "" {
			msg.Text = "Укажите город после команды. Пример: /weather Москва"
			return
		}
		fetchAndSendWeather(args, msg)
	default:
		msg.Text = "Неизвестная команда. Попробуйте /start"
	}
}

func handleTextMessage(update tgbotapi.Update, msg *tgbotapi.MessageConfig) {
	text := strings.TrimSpace(update.Message.Text)
	if text == "" {
		msg.Text = "Введите название города"
		return
	}
	fetchAndSendWeather(text, msg)
}

func fetchAndSendWeather(city string, msg *tgbotapi.MessageConfig) {
	apiURL := os.Getenv("WEATHER_API_URL")
	if apiURL == "" {
		msg.Text = "❌ WEATHER_API_URL не задан в окружении"
		return
	}

	// URL-энкодинг города, чтобы поддерживать русские символы
	cityEncoded := url.QueryEscape(city)
	fullURL := fmt.Sprintf("%s?city=%s", apiURL, cityEncoded)

	resp, err := http.Get(fullURL)
	if err != nil {
		msg.Text = fmt.Sprintf("❌ Ошибка запроса к API: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		msg.Text = fmt.Sprintf("❌ API вернуло ошибку: %d", resp.StatusCode)
		return
	}

	var w Weather
	if err := json.NewDecoder(resp.Body).Decode(&w); err != nil {
		msg.Text = fmt.Sprintf("❌ Ошибка декодирования JSON: %v", err)
		return
	}

	msg.Text = fmt.Sprintf(
		"🌤 Погода в %s:\n• Температура: %.1f°C\n• Ощущается как: %.1f°C\n• Влажность: %d%%\n• Состояние: %s",
		w.City, w.Temp, w.FeelsLike, w.Humidity, w.Condition,
	)
}
