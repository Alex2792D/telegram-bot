package main

import (
	"bytes"
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

type UserData struct {
	UserID    int64  `json:"user_id"`
	UserName  string `json:"username"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

func main() {
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

	webhookURL := os.Getenv("WEBHOOK_URL")
	if webhookURL == "" {
		log.Fatal("❌ WEBHOOK_URL не задан")
	}

	webhookConfig, err := tgbotapi.NewWebhook(webhookURL)
	if err != nil {
		log.Fatal("❌ Ошибка создания WebhookConfig:", err)
	}

	_, err = bot.Request(webhookConfig)
	if err != nil {
		log.Fatal("❌ Ошибка установки webhook:", err)
	}

	updates := bot.ListenForWebhook("/bot")

	go func() {
		log.Printf("📡 HTTP сервер слушает :%s", port)
		if err := http.ListenAndServe(":"+port, nil); err != nil {
			log.Fatal("❌ HTTP сервер упал:", err)
		}
	}()

	log.Println("🚀 Бот запущен и ждет сообщений")

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
			log.Printf("⚠️ Ошибка отправки: %v", err)
		}
	}
}

func handleCommand(update tgbotapi.Update, msg *tgbotapi.MessageConfig) {
	switch update.Message.Command() {
	case "start":
		msg.Text = "Привет! Я погодный бот. Используй /weather <город>"
		sendUserData(update.Message.From)
	case "help":
		msg.Text = "Я показываю погоду. Используй /weather <город>"
	case "weather":
		city := update.Message.CommandArguments()
		if city == "" {
			msg.Text = "❌ Укажи город после команды /weather"
			return
		}
		fetchAndSendWeather(city, msg)
	default:
		msg.Text = "❌ Неизвестная команда"
	}
}

func handleTextMessage(update tgbotapi.Update, msg *tgbotapi.MessageConfig) {
	text := strings.TrimSpace(update.Message.Text)
	if text == "" {
		msg.Text = "❌ Пожалуйста, введите город"
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

	resp, err := http.Get(fmt.Sprintf("%s?city=%s", apiURL, city))
	if err != nil {
		msg.Text = fmt.Sprintf("❌ Ошибка запроса к API: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		msg.Text = fmt.Sprintf("❌ API вернуло ошибку: %d", resp.StatusCode)
		return
	}

	var weather Weather
	if err := json.NewDecoder(resp.Body).Decode(&weather); err != nil {
		msg.Text = fmt.Sprintf("❌ Ошибка декодирования JSON: %v", err)
		return
	}

	msg.Text = fmt.Sprintf(
		"🌤 Погода в %s:\n• Температура: %.1f°C\n• Ощущается как: %.1f°C\n• Влажность: %d%%\n• Состояние: %s",
		weather.City, weather.Temp, weather.FeelsLike, weather.Humidity, weather.Condition,
	)
}

func sendUserData(user *tgbotapi.User) {
	if user == nil {
		return
	}

	data := UserData{
		UserID:    user.ID,
		UserName:  user.UserName,
		FirstName: user.FirstName,
		LastName:  user.LastName,
	}

	serviceURL := os.Getenv("USER_SERVICE_URL")
	if serviceURL == "" {
		log.Println("❌ USER_SERVICE_URL не задан, данные пользователя не отправлены")
		return
	}

	payload, _ := json.Marshal(data)
	resp, err := http.Post(serviceURL, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		log.Printf("❌ Ошибка отправки данных пользователя: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("❌ Сервис вернул код: %d", resp.StatusCode)
	}
}
