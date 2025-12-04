package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
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
	// Загружаем .env локально, если это не Render
	if os.Getenv("RENDER") == "" {
		godotenv.Load()
	}

	// -----------------------------
	// Telegram Bot
	// -----------------------------
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

	// -----------------------------
	// Webhook
	// -----------------------------
	webhookURL := os.Getenv("WEBHOOK_URL") // Например: https://telegram-bot-kuk3.onrender.com/bot
	if webhookURL == "" {
		log.Fatal("❌ WEBHOOK_URL не задан")
	}

	wh, err := tgbotapi.NewWebhook(webhookURL)
	if err != nil {
		log.Fatal("❌ Ошибка создания webhook:", err)
	}

	_, err = bot.Request(wh)
	if err != nil {
		log.Fatal("❌ Ошибка установки webhook:", err)
	}

	updates := bot.ListenForWebhook("/bot")

	// -----------------------------
	// HTTP сервер для webhook
	// -----------------------------
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	go func() {
		log.Printf("📡 HTTP сервер слушает :%s", port)
		if err := http.ListenAndServe(":"+port, nil); err != nil {
			log.Fatal(err)
		}
	}()

	log.Println("🚀 Бот запущен и ждет сообщений")

	// -----------------------------
	// Обработка обновлений
	// -----------------------------
	for update := range updates {
		if update.Message == nil {
			continue
		}

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
	switch update.Message.Command() {
	case "start":
		msg.Text = "Привет! Я погодный бот. Используй /weather <город>"
	case "help":
		msg.Text = "Я показываю погоду. Используй /weather <город>"
	case "weather":
		city := update.Message.CommandArguments()
		if city == "" {
			msg.Text = "Укажи город после команды /weather"
			return
		}
		fetchAndSendWeather(city, msg)
	default:
		msg.Text = "Неизвестная команда"
	}
}

func handleTextMessage(update tgbotapi.Update, msg *tgbotapi.MessageConfig) {
	text := strings.TrimSpace(update.Message.Text)
	if text == "" {
		msg.Text = "Пожалуйста, введите город"
		return
	}
	fetchAndSendWeather(text, msg)
}

func fetchAndSendWeather(city string, msg *tgbotapi.MessageConfig) {
	apiURL := os.Getenv("WEATHER_API_URL")
	if apiURL == "" {
		msg.Text = "❌ WEATHER_API_URL не задан"
		return
	}

	// Формируем URL запроса
	url := fmt.Sprintf("%s?city=%s", apiURL, city)

	resp, err := http.Get(url)
	if err != nil || resp.StatusCode != 200 {
		msg.Text = fmt.Sprintf("❌ Ошибка получения погоды: %v", err)
		return
	}
	defer resp.Body.Close()

	var weather Weather
	if err := json.NewDecoder(resp.Body).Decode(&weather); err != nil {
		msg.Text = fmt.Sprintf("❌ Ошибка декодирования ответа: %v", err)
		return
	}

	msg.Text = fmt.Sprintf(
		"🌤 Погода в %s:\n• Температура: %.1f°C\n• Ощущается как: %.1f°C\n• Влажность: %d%%\n• Состояние: %s",
		weather.City,
		weather.Temp,
		weather.FeelsLike,
		weather.Humidity,
		weather.Condition,
	)
}
