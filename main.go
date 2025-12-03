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
	Temp      float64 `json:"temp"`
	FeelsLike float64 `json:"feels_like"`
	Humidity  int     `json:"humidity"`
	Condition string  `json:"condition"`
}

func main() {
	if os.Getenv("RENDER") == "" {
		if err := godotenv.Load(); err != nil {
			log.Printf("⚠️ .env не найден (локально) — используем окружение")
		}
	}

	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		log.Fatal("❌ TELEGRAM_BOT_TOKEN не задан. Установите его в Render → Environment или в .env")
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
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("✅ OK\nBot: @" + bot.Self.UserName))
	})

	go func() {
		log.Printf("📡 HTTP сервер слушает :%s", port)
		if err := http.ListenAndServe(":"+port, nil); err != nil && err != http.ErrServerClosed {
			log.Fatal("❌ HTTP сервер упал:", err)
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

		switch {
		case update.Message.IsCommand():
			handleCommand(update, &msg)
		default:
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
		msg.Text = "Привет! Я погодный бот. Доступные команды:\n" +
			"/start — Начало\n" +
			"/help — Справка\n" +
			"/weather <город> — Узнать погоду (например: /погода Москва)"
	case "help":
		msg.Text = "Я показываю погоду по запросу.\n" +
			"Используйте:\n" +
			"- /погода <город> (например: /погода Санкт-Петербург)\n" +
			"- Или просто напишите название города"
	case "weather":
		args := update.Message.CommandArguments()
		if args == "" {
			msg.Text = "Укажите город после команды /погода. Пример: /погода Казань"
			return
		}
		fetchAndSendWeather(args, msg)
	default:
		msg.Text = "Неизвестная команда. Попробуйте /start или /help."
	}
}

func handleTextMessage(update tgbotapi.Update, msg *tgbotapi.MessageConfig) {
	text := strings.TrimSpace(update.Message.Text)
	if text == "" {
		msg.Text = "Пожалуйста, введите название города."
		return
	}

	if strings.HasPrefix(text, "погода ") {
		city := strings.TrimPrefix(text, "погода ")
		fetchAndSendWeather(city, msg)
		return
	}

	fetchAndSendWeather(text, msg)
}

func fetchAndSendWeather(city string, msg *tgbotapi.MessageConfig) {
	weather, err := fetchWeatherFromAPI(city)
	if err != nil {
		msg.Text = fmt.Sprintf("Ошибка получения погоды: %v", err)
		return
	}
	msg.Text = formatWeatherResponse(weather)
}

func fetchWeatherFromAPI(city string) (*Weather, error) {
	apiURL := os.Getenv("WEATHER_API_URL")
	if apiURL == "" {
		return nil, fmt.Errorf("WEATHER_API_URL не задан в окружении")
	}

	url := fmt.Sprintf("%s?city=%s", apiURL, city)
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("ошибка запроса к API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API вернуло ошибку: %d", resp.StatusCode)
	}

	var weather Weather
	if err := json.NewDecoder(resp.Body).Decode(&weather); err != nil {
		return nil, fmt.Errorf("ошибка декодирования JSON: %w", err)
	}

	return &weather, nil
}

func formatWeatherResponse(w *Weather) string {
	return fmt.Sprintf(
		"🌤 Погода в %s:\n"+
			"• Температура: %.1f°C\n"+
			"• Ощущается как: %.1f°C\n"+
			"• Влажность: %d%%\n"+
			"• Состояние: %s",
		w.City,
		w.Temp,
		w.FeelsLike,
		w.Humidity,
		w.Condition,
	)
}
