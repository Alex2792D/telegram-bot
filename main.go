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
	if os.Getenv("RENDER") == "" {
		_ = godotenv.Load()
	}

	token := strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN"))
	webhookURL := strings.TrimSpace(os.Getenv("WEBHOOK_URL"))
	weatherAPI := strings.TrimSpace(os.Getenv("WEATHER_API_URL"))

	if token == "" {
		log.Fatal("❌ TELEGRAM_BOT_TOKEN не задан")
	}
	if webhookURL == "" {
		log.Fatal("❌ WEBHOOK_URL не задан")
	}
	if weatherAPI == "" {
		log.Fatal("❌ WEATHER_API_URL не задан")
	}

	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Fatal("❌ Ошибка инициализации бота:", err)
	}
	log.Printf("✅ Авторизован как @%s", bot.Self.UserName)

	// ✅ 1. Создаём конфиг webhook
	webhookConfig, err := tgbotapi.NewWebhook(webhookURL)
	if err != nil {
		log.Fatal("❌ Ошибка создания WebhookConfig:", err)
	}

	// ✅ 2. Устанавливаем webhook через Request (для v5.5 и ниже)
	_, err = bot.Request(webhookConfig)
	if err != nil {
		log.Fatal("❌ Ошибка установки webhook:", err)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// ✅ 3. Создаём канал для обновлений (ручное управление)
	updates := make(chan tgbotapi.Update, 100)

	// ✅ 4. Регистрируем HTTP-обработчик для /bot
	mux := http.NewServeMux()

	mux.HandleFunc("/bot", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Only POST allowed", http.StatusMethodNotAllowed)
			return
		}

		var receivedUpdates []tgbotapi.Update
		if err := json.NewDecoder(r.Body).Decode(&receivedUpdates); err != nil {
			log.Printf("❌ Decode error: %v", err)
			http.Error(w, "Bad JSON", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		// Отправляем каждое обновление в канал
		for _, update := range receivedUpdates {
			select {
			case updates <- update:
				// OK
			default:
				log.Printf("⚠️ Канал переполнен, пропускаем update ID=%d", update.UpdateID)
			}
		}

		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("✅ Bot is running\nPOST /bot for webhook"))
	})

	// ✅ Запускаем сервер в фоне
	go func() {
		log.Printf("📡 HTTP сервер слушает :%s", port)
		if err := http.ListenAndServe(":"+port, mux); err != nil {
			log.Fatal("❌ HTTP сервер упал:", err)
		}
	}()

	log.Println("🚀 Бот запущен. Ожидание сообщений...")

	// ✅ Основной цикл обработки — читаем из канала
	log.Println("🟢 Цикл обработки обновлений запущен")
	for update := range updates {
		if update.Message == nil {
			continue
		}

		chatID := update.Message.Chat.ID
		log.Printf("📥 [%d] %s", chatID, update.Message.Text)

		msg := tgbotapi.NewMessage(chatID, "")

		if update.Message.IsCommand() {
			handleCommand(update, &msg, weatherAPI)
		} else {
			handleTextMessage(update, &msg, weatherAPI)
		}

		if _, err := bot.Send(msg); err != nil {
			log.Printf("⚠️ Ошибка отправки: %v", err)
		}
	}
}

// --- Остальные функции без изменений ---
func handleCommand(update tgbotapi.Update, msg *tgbotapi.MessageConfig, weatherAPI string) {
	cmd := update.Message.Command()
	args := update.Message.CommandArguments()

	switch cmd {
	case "start":
		msg.Text = "Привет! 🌤 Я — погодный бот.\n\n" +
			"🔹 Чтобы узнать погоду — отправь название города.\n" +
			"🔹 Или используй команду: /weather Москва"
	case "help":
		msg.Text = "📌 Как пользоваться:\n" +
			"• Просто напиши: `Москва`\n" +
			"• Или: `/weather London`\n" +
			"Поддерживаются русские и английские названия."
	case "weather":
		if city := strings.TrimSpace(args); city != "" {
			fetchAndSendWeather(city, msg, weatherAPI)
		} else {
			msg.Text = "❓ Укажи город, например: `/weather Moscow`"
		}
	default:
		msg.Text = "❌ Неизвестная команда. Попробуй /start"
	}
}

func handleTextMessage(update tgbotapi.Update, msg *tgbotapi.MessageConfig, weatherAPI string) {
	if city := strings.TrimSpace(update.Message.Text); city != "" {
		fetchAndSendWeather(city, msg, weatherAPI)
	} else {
		msg.Text = "🤔 Пустое сообщение. Напиши город, например: `Москва`"
	}
}

func fetchAndSendWeather(city string, msg *tgbotapi.MessageConfig, weatherAPI string) {
	log.Printf("🔍 Запрашиваю погоду для: %q", city)

	resp, err := http.Get(fmt.Sprintf("%s?city=%s", weatherAPI, city))
	if err != nil {
		msg.Text = "⚠️ Ошибка подключения к сервису погоды"
		log.Printf("🌐 HTTP error: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		msg.Text = fmt.Sprintf("⚠️ Сервис погоды вернул %d", resp.StatusCode)
		log.Printf("📡 HTTP status: %d", resp.StatusCode)
		return
	}

	var weather Weather
	if err := json.NewDecoder(resp.Body).Decode(&weather); err != nil {
		msg.Text = "❌ Ошибка разбора ответа погоды"
		log.Printf("🧩 JSON decode error: %v", err)
		return
	}

	if weather.City == "" {
		msg.Text = "🌍 Город не найден. Попробуй: `Moscow`, `London`"
		return
	}

	msg.Text = fmt.Sprintf(
		"🌤 Погода в %s:\n"+
			"• Температура: %.1f°C\n"+
			"• Ощущается как: %.1f°C\n"+
			"• Влажность: %d%%\n"+
			"• Состояние: %s",
		weather.City,
		weather.Temp,
		weather.FeelsLike,
		weather.Humidity,
		weather.Condition,
	)
	log.Printf("✅ Отправлена погода для %s", weather.City)
}
