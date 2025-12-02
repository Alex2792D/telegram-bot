package main

import (
	"log"
	"net/http"
	"os"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
)

func main() {
	// ——— 1. Загружаем .env ЛОКАЛЬНО ———
	// На Render .env нет — и не нужно. godotenv проигнорирует ошибку.
	if os.Getenv("RENDER") == "" {
		// Считаем, что локально — грузим .env
		if err := godotenv.Load(); err != nil {
			log.Printf("⚠️ .env не найден (локально) — используем окружение")
		}
	}

	// ——— 2. Получаем токен — из окружения (приоритет!) ———
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		log.Fatal("❌ TELEGRAM_BOT_TOKEN не задан. Установите его в Render → Environment или в .env")
	}

	// ——— 3. Инициализация бота ———
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Fatal("❌ Ошибка инициализации бота:", err)
	}
	bot.Debug = true
	log.Printf("✅ Авторизован как @%s", bot.Self.UserName)

	// ——— 4. ОБЯЗАТЕЛЬНО: HTTP-сервер для Render ———
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080" // fallback для локального запуска
	}

	// Health-check эндпоинт
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("✅ OK\nBot: @" + bot.Self.UserName))
	})

	// Запускаем HTTP в фоне — НЕ БЛОКИРУЕМ main!
	go func() {
		log.Printf("📡 HTTP сервер слушает :%s", port)
		if err := http.ListenAndServe(":"+port, nil); err != nil && err != http.ErrServerClosed {
			log.Fatal("❌ HTTP сервер упал:", err)
		}
	}()

	// ——— 5. Long polling — как у тебя было ———
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil {
			continue
		}

		log.Printf("[%s] %s", update.Message.From.UserName, update.Message.Text)

		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")
		switch update.Message.Text {
		case "/start":
			msg.Text = "Привет! Я ваш бот. Используйте /help для справки."
		case "/help":
			msg.Text = "Доступные команды:\n/start — Начало\n/help — Справка"
		default:
			msg.Text = "Я пока умею только отвечать на /start и /help."
		}

		if _, err := bot.Send(msg); err != nil {
			log.Printf("⚠️ Ошибка отправки: %v", err)
		}
	}
}
