package main

import (
	"log"
	"os"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv" // Добавляем импорт
)

func main() {
	// Загружаем переменные из .env (если файл существует)
	if err := godotenv.Load(); err != nil {
		log.Printf("Не удалось загрузить .env: %v (продолжаем работу)", err)
	}

	// Получаем токен из переменной окружения
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		log.Fatal("Токен не найден! Установите переменную TELEGRAM_BOT_TOKEN в .env или окружении")
	}

	// Инициализация бота
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Fatal(err)
	}

	bot.Debug = true
	log.Printf("Авторизован как %s", bot.Self.UserName)

	// Настройка получения обновлений
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)

	// Обработчик сообщений
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
			log.Println(err)
		}
	}
}
