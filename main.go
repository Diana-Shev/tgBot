// package main

// import (
// 	"bytes"         // для работы с байтовыми буферами(нужно при отправке запроса)
// 	"encoding/json" // преобразует данные в JSON и обратно
// 	"fmt"           // форматирование строк
// 	"io/ioutil"     // читает ответы от сервера
// 	"log"
// 	"net/http" // отправляет http- запросы
// 	"os"       // для работы с переменными окружения (.env)
// 	"strings"
// 	"time"

// 	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
// 	"github.com/joho/godotenv"
// )

// // структуры, которые описывают, как будет выглядеть JSON-запрос

// type Message struct {
// 	Role string `json:"role"`
// 	Text string `json:"text"`
// }

// type CompletionOptions struct {
// 	Stream      bool    `json:"stream"`      // нужно ли получать ответ по частям, пока не используем (false)
// 	Temperature float64 `json:"temperature"` // творчество модели, шаг 0.3, 0,0 - строго, 1,0 хаотично
// 	MaxTokens   int     `json:"maxTokens"`   // макс длина ответа в токенах
// }

// type GPTRequest struct {
// 	ModelUri          string            `json:"modelUri"`          // адрес модели
// 	CompletionOptions CompletionOptions `json:"completionOptions"` // настройки генерации
// 	Messages          []Message         `json:"messages"`          // сообещния в чате
// }

// type Session struct {
// 	Topics         []string // 10 тем от GPT
// 	SelectedTopics []int    // индексы выбранных тем
// }

// var sessions = make(map[int64]*Session) // ключ chatID

// func askYandexGPT(userText string) (string, error) { // отправляет сообщение в GPT и получает ответ
// 	apiKey := os.Getenv("YANDEX_API_KEY")
// 	folderID := os.Getenv("YANDEX_FOLDER_ID")

// 	url := "https://llm.api.cloud.yandex.net/foundationModels/v1/completion" //url по которому делаем запрос GPT

// 	requestBody := GPTRequest{ // формируем JSON- запрос
// 		ModelUri: fmt.Sprintf("gpt://%s/yandexgpt-lite", folderID),
// 		CompletionOptions: CompletionOptions{
// 			Stream:      false,
// 			Temperature: 0.7,
// 			MaxTokens:   300, // 100
// 		},
// 		Messages: []Message{
// 			{Role: "user", Text: userText},
// 		},
// 	}

// 	jsonData, err := json.Marshal(requestBody) //преобразуем структуру запроса GPTRequest в JSON (в байтах), чтобы отправить в тело запроса
// 	if err != nil {
// 		return "", err
// 	}

// 	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData)) //создаю POST- запрос с этим JSON-ом
// 	if err != nil {
// 		return "", err
// 	}

// 	// добавляем заголовки, как в Postman
// 	req.Header.Set("Authorization", "Api-Key "+apiKey)
// 	req.Header.Set("Content-Type", "application/json")
// 	req.Header.Set("x-folder-id", folderID)

// 	client := &http.Client{}    // отправляем запрос в YGPT
// 	resp, err := client.Do(req) // получаем ответ
// 	if err != nil {
// 		return "", err
// 	}
// 	defer resp.Body.Close()

// 	body, _ := ioutil.ReadAll(resp.Body)

// 	var result map[string]interface{} // преобразуем JSON-ответ от GPT в map
// 	err = json.Unmarshal(body, &result)
// 	if err != nil {
// 		return "", err
// 	}

// 	// Парсим ответ
// 	text := "Ответ не получен"
// 	if r, ok := result["result"].(map[string]interface{}); ok {
// 		if alternatives, ok := r["alternatives"].([]interface{}); ok && len(alternatives) > 0 {
// 			msg := alternatives[0].(map[string]interface{})["message"].(map[string]interface{})
// 			text = msg["text"].(string)
// 		}
// 	}

// 	return text, nil
// }

// func main() {
// 	err := godotenv.Load() //загружаем переменные из .env
// 	if err != nil {
// 		log.Fatal("Ошибка загрузки .env файла")
// 	}

// 	InitDatabase() // подключаем БД

// 	botToken := os.Getenv("TELEGRAM_BOT_TOKEN") //подключаемся к tgBot API с токеном
// 	bot, err := tgbotapi.NewBotAPI(botToken)
// 	if err != nil {
// 		log.Panic(err)
// 	}

// 	log.Printf(" Бот запущен: %s", bot.Self.UserName)

// 	u := tgbotapi.NewUpdate(0) // готовим бота к приему новых сообщений из чата
// 	u.Timeout = 60
// 	updates := bot.GetUpdatesChan(u)

// 	for update := range updates { // запускаем бесконечный цикл, обработка каждого входящего сообщения
// 		if update.Message != nil && update.Message.IsCommand() {
// 			switch update.Message.Command() {
// 			case "start":
// 				reply := "⏳ Генерирую темы, подожди секунду..."

// 				msg := tgbotapi.NewMessage(update.Message.Chat.ID, reply)
// 				bot.Send(msg)

// 				// Запрос к GPT: сгенерируй 10 тем
// 				topicsText, err := askYandexGPT("Сгенерируй 10 интересных, разнообразных тем для историй. Ответ выдай в виде пронумерованного списка из 2 или 3 слов. Например: кулинария. Интересный факт о пирожках.")
// 				if err != nil {
// 					bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "❌ Ошибка генерации тем: "+err.Error()))
// 					fmt.Println("GPT ERROR:", err)
// 					continue
// 				}

// 				fmt.Println("GPT ОТВЕТ:", topicsText)

// 				if err != nil {
// 					bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "❌ Ошибка генерации тем: "+err.Error()))
// 					continue
// 				}

// 				// Парсим список тем из ответа GPT
// 				lines := strings.Split(topicsText, "\n")
// 				var topics []string
// 				for _, line := range lines {
// 					line = strings.TrimSpace(line)
// 					if line == "" {
// 						continue
// 					}
// 					// Удаляем нумерацию (1. Тема, 2. Тема...)
// 					if i := strings.Index(line, "."); i != -1 {
// 						line = strings.TrimSpace(line[i+1:])
// 					}
// 					topics = append(topics, line)
// 				}

// 				// Ограничим до 10 на всякий случай
// 				if len(topics) > 10 {
// 					topics = topics[:10]
// 				}

// 				chatID := update.Message.Chat.ID
// 				sessions[chatID] = &Session{
// 					Topics:         topics,
// 					SelectedTopics: []int{},
// 				}

// 				// Генерируем кнопки для тем
// 				var rows [][]tgbotapi.InlineKeyboardButton
// 				for i, topic := range topics {
// 					btn := tgbotapi.NewInlineKeyboardButtonData(topic, fmt.Sprintf("topic_%d", i))
// 					rows = append(rows, tgbotapi.NewInlineKeyboardRow(btn))
// 				}

// 				// Добавляем кнопку Restart
// 				restartBtn := tgbotapi.NewInlineKeyboardButtonData("🔁 Сгенерировать заново", "restart")
// 				rows = append(rows, tgbotapi.NewInlineKeyboardRow(restartBtn))

// 				keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)
// 				msgWithButtons := tgbotapi.NewMessage(update.Message.Chat.ID, "Выбери 3 интересных темы:")
// 				msgWithButtons.ReplyMarkup = keyboard

// 				_, sendErr := bot.Send(msgWithButtons)
// 				if sendErr != nil {
// 					fmt.Println("❌ Ошибка отправки сообщения:", sendErr)
// 				}

// 				continue
// 			}
// 		}

// 		if update.CallbackQuery != nil {
// 			callback := update.CallbackQuery
// 			data := callback.Data // что именно нажал пользователь

// 			//  Обработка кнопки "Сгенерировать заново"
// 			if data == "restart" {
// 				msg := tgbotapi.NewMessage(callback.Message.Chat.ID, "🔁 Генерирую новые темы...")
// 				bot.Send(msg)

// 				topicsText, err := askYandexGPT("Сгенерируй 10 интересных, разнообразных тем для историй. Ответ выдай в виде пронумерованного списка из 2 или 3 слов. Например: кулинария. Интересный факт о пирожках")
// 				if err != nil {
// 					bot.Send(tgbotapi.NewMessage(callback.Message.Chat.ID, "❌ Ошибка генерации тем: "+err.Error()))
// 					return
// 				}

// 				// Разбиваем ответ на строки
// 				lines := strings.Split(topicsText, "\n")
// 				var topics []string
// 				for _, line := range lines {
// 					line = strings.TrimSpace(line)
// 					if line == "" {
// 						continue
// 					}
// 					if i := strings.Index(line, "."); i != -1 {
// 						line = strings.TrimSpace(line[i+1:])
// 					}
// 					topics = append(topics, line)
// 				}

// 				if len(topics) > 10 {
// 					topics = topics[:10]
// 				}

// 				var rows [][]tgbotapi.InlineKeyboardButton
// 				for i, topic := range topics {
// 					btn := tgbotapi.NewInlineKeyboardButtonData(topic, fmt.Sprintf("topic_%d", i))
// 					rows = append(rows, tgbotapi.NewInlineKeyboardRow(btn))
// 				}

// 				restartBtn := tgbotapi.NewInlineKeyboardButtonData("🔁 Сгенерировать заново", "restart")
// 				rows = append(rows, tgbotapi.NewInlineKeyboardRow(restartBtn))

// 				chatID := callback.Message.Chat.ID
// 				sessions[chatID] = &Session{
// 					Topics:         topics,
// 					SelectedTopics: []int{},
// 				}

// 				keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)
// 				msgWithButtons := tgbotapi.NewMessage(callback.Message.Chat.ID, "Выбери 3 интересных темы:")
// 				msgWithButtons.ReplyMarkup = keyboard
// 				bot.Send(msgWithButtons)

// 				return
// 			}

// 			// временно, чтобы проверить нажимается ли кнопка выбранной темы

// 			if strings.HasPrefix(data, "topic_") {
// 				ack := tgbotapi.NewCallback(callback.ID, "")
// 				bot.Request(ack)

// 				chatID := callback.Message.Chat.ID
// 				session, exists := sessions[chatID]
// 				if !exists || len(session.Topics) == 0 {
// 					bot.Send(tgbotapi.NewMessage(chatID, "⚠️ Темы не найдены. Нажмите /start."))
// 					return
// 				}

// 				i := 0
// 				fmt.Sscanf(data, "topic_%d", &i)

// 				// Уже выбрана?
// 				for _, v := range session.SelectedTopics {
// 					if v == i {
// 						bot.Send(tgbotapi.NewMessage(chatID, "⚠️ Эта тема уже выбрана."))
// 						return
// 					}
// 				}

// 				session.SelectedTopics = append(session.SelectedTopics, i)

// 				if len(session.SelectedTopics) < 3 {
// 					topic := session.Topics[i]
// 					reply := fmt.Sprintf("✅ Тема добавлена: %s (%d из 3)", topic, len(session.SelectedTopics))
// 					bot.Send(tgbotapi.NewMessage(chatID, reply))
// 					return
// 				}

// 				// Когда выбрано 3 темы
// 				var rows [][]tgbotapi.InlineKeyboardButton
// 				for _, idx := range session.SelectedTopics {
// 					topic := session.Topics[idx]
// 					btn := tgbotapi.NewInlineKeyboardButtonData(topic, fmt.Sprintf("chosen_%d", idx))
// 					rows = append(rows, tgbotapi.NewInlineKeyboardRow(btn))
// 				}

// 				keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)
// 				msg := tgbotapi.NewMessage(chatID, "Теперь выбери одну тему, по которой хочешь услышать историю:")
// 				msg.ReplyMarkup = keyboard
// 				bot.Send(msg)

// 				return
// 			}

// 		}

// 		if update.Message == nil { //если пришло НЕ текстовое сообщение(типо фото), пропускаем
// 			continue
// 		}

// 		userMessage := update.Message.Text          // получаем текст от пользователя
// 		replyText, err := askYandexGPT(userMessage) // отправляет его в GPT
// 		status := "успех"
// 		if err != nil {
// 			replyText = " Ошибка: " + err.Error()
// 			status = "Ошибка"
// 		}

// 		// сохраняем запрос в БД
// 		_, dbErr := DB.Exec(`
// 			INSERT INTO requests (username, timestamp, user_text, gpt_response, status)
// 			VALUES ($1, $2, $3, $4, $5)
// 		`, update.Message.From.UserName, time.Now().Format(time.RFC3339), userMessage, replyText, status)

// 		if dbErr != nil {
// 			log.Println("Ошибка записи в БД:", dbErr)
// 		}
// 		msg := tgbotapi.NewMessage(update.Message.Chat.ID, replyText) // отправляем ответ в telegram обратно пользователю
// 		bot.Send(msg)
// 	}
// }

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
)

// Структуры для запроса к Yandex GPT

type Message struct {
	Role string `json:"role"`
	Text string `json:"text"`
}

type CompletionOptions struct {
	Stream      bool    `json:"stream"`
	Temperature float64 `json:"temperature"`
	MaxTokens   int     `json:"maxTokens"`
}

type GPTRequest struct {
	ModelUri          string            `json:"modelUri"`
	CompletionOptions CompletionOptions `json:"completionOptions"`
	Messages          []Message         `json:"messages"`
}

// Структура сессии пользователя

type Session struct {
	Topics         []string
	SelectedTopics []int
}

var sessions = make(map[int64]*Session) // хранилище сессий пользователей

func askYandexGPT(userText string) (string, error) {
	apiKey := os.Getenv("YANDEX_API_KEY")
	folderID := os.Getenv("YANDEX_FOLDER_ID")
	url := "https://llm.api.cloud.yandex.net/foundationModels/v1/completion"

	requestBody := GPTRequest{
		ModelUri: fmt.Sprintf("gpt://%s/yandexgpt-lite", folderID),
		CompletionOptions: CompletionOptions{
			Stream:      false,
			Temperature: 0.7,
			MaxTokens:   300,
		},
		Messages: []Message{{Role: "user", Text: userText}},
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Api-Key "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-folder-id", folderID)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)

	var result map[string]interface{}
	err = json.Unmarshal(body, &result)
	if err != nil {
		return "", err
	}

	text := "Ответ не получен"
	if r, ok := result["result"].(map[string]interface{}); ok {
		if alternatives, ok := r["alternatives"].([]interface{}); ok && len(alternatives) > 0 {
			msg := alternatives[0].(map[string]interface{})["message"].(map[string]interface{})
			text = msg["text"].(string)
		}
	}

	return text, nil
}

func main() {
	_ = godotenv.Load()

	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Panic(err)
	}

	log.Printf(" Бот запущен: %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message != nil && update.Message.IsCommand() {
			switch update.Message.Command() {
			case "start":
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "⏳ Генерирую темы, подожди секунду...")
				bot.Send(msg)

				topicsText, err := askYandexGPT("Сгенерируй 10 интересных, разнообразных тем для историй. Ответ выдай в виде пронумерованного списка из 2 или 3 слов.")
				if err != nil {
					bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "❌ Ошибка генерации тем: "+err.Error()))
					continue
				}

				lines := strings.Split(topicsText, "\n")
				var topics []string
				for _, line := range lines {
					line = strings.TrimSpace(line)
					if line == "" {
						continue
					}
					if i := strings.Index(line, "."); i != -1 {
						line = strings.TrimSpace(line[i+1:])
					}
					topics = append(topics, line)
				}

				if len(topics) > 10 {
					topics = topics[:10]
				}

				sessions[update.Message.Chat.ID] = &Session{Topics: topics, SelectedTopics: []int{}}

				var rows [][]tgbotapi.InlineKeyboardButton
				for i, topic := range topics {
					btn := tgbotapi.NewInlineKeyboardButtonData(topic, fmt.Sprintf("topic_%d", i))
					rows = append(rows, tgbotapi.NewInlineKeyboardRow(btn))
				}
				restartBtn := tgbotapi.NewInlineKeyboardButtonData("🔁 Сгенерировать заново", "restart")
				rows = append(rows, tgbotapi.NewInlineKeyboardRow(restartBtn))

				keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)
				msgWithButtons := tgbotapi.NewMessage(update.Message.Chat.ID, "Выбери 3 интересных темы:")
				msgWithButtons.ReplyMarkup = keyboard
				bot.Send(msgWithButtons)
			}
		}

		if update.CallbackQuery != nil {
			callback := update.CallbackQuery
			data := callback.Data
			chatID := callback.Message.Chat.ID
			ack := tgbotapi.NewCallback(callback.ID, "")
			bot.Request(ack)

			if data == "restart" {
				topicsText, err := askYandexGPT("Сгенерируй 10 интересных, разнообразных тем для историй. Ответ выдай в виде пронумерованного списка из 2 или 3 слов.")
				if err != nil {
					bot.Send(tgbotapi.NewMessage(chatID, "❌ Ошибка генерации тем: "+err.Error()))
					continue
				}

				lines := strings.Split(topicsText, "\n")
				var topics []string
				for _, line := range lines {
					line = strings.TrimSpace(line)
					if line == "" {
						continue
					}
					if i := strings.Index(line, "."); i != -1 {
						line = strings.TrimSpace(line[i+1:])
					}
					topics = append(topics, line)
				}

				if len(topics) > 10 {
					topics = topics[:10]
				}

				sessions[chatID] = &Session{Topics: topics, SelectedTopics: []int{}}

				var rows [][]tgbotapi.InlineKeyboardButton
				for i, topic := range topics {
					btn := tgbotapi.NewInlineKeyboardButtonData(topic, fmt.Sprintf("topic_%d", i))
					rows = append(rows, tgbotapi.NewInlineKeyboardRow(btn))
				}
				restartBtn := tgbotapi.NewInlineKeyboardButtonData("🔁 Сгенерировать заново", "restart")
				rows = append(rows, tgbotapi.NewInlineKeyboardRow(restartBtn))

				keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)
				msg := tgbotapi.NewMessage(chatID, "Выбери 3 интересных темы:")
				msg.ReplyMarkup = keyboard
				bot.Send(msg)
				continue
			}

			if strings.HasPrefix(data, "topic_") {
				index := 0
				fmt.Sscanf(data, "topic_%d", &index)
				session, exists := sessions[chatID]
				if !exists {
					bot.Send(tgbotapi.NewMessage(chatID, "Сессия не найдена. Введите /start."))
					continue
				}
				for _, v := range session.SelectedTopics {
					if v == index {
						bot.Send(tgbotapi.NewMessage(chatID, "⚠️ Эта тема уже выбрана."))
						return
					}
				}
				session.SelectedTopics = append(session.SelectedTopics, index)
				if len(session.SelectedTopics) < 3 {
					topic := session.Topics[index]
					bot.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("✅ Тема добавлена: %s (%d из 3)", topic, len(session.SelectedTopics))))
					continue
				}
				var rows [][]tgbotapi.InlineKeyboardButton
				for _, idx := range session.SelectedTopics {
					topic := session.Topics[idx]
					btn := tgbotapi.NewInlineKeyboardButtonData(topic, fmt.Sprintf("chosen_%d", idx))
					rows = append(rows, tgbotapi.NewInlineKeyboardRow(btn))
				}
				keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)
				msg := tgbotapi.NewMessage(chatID, "Теперь выбери одну тему, по которой хочешь услышать историю:")
				msg.ReplyMarkup = keyboard
				bot.Send(msg)
				continue
			}
			// Обработка выбранной темы для истории пока не реализована
		}
	}
}
