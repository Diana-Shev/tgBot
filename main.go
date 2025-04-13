// main.go

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

// Структуры для запроса к YandexGPT

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

// Структура для хранения сессии пользователя

type Session struct {
	Topics         []string
	SelectedTopics []int
	ChosenIndex    int
}

var sessions = make(map[int64]*Session)

func askYandexGPT(userText string, maxTokens int) (string, error) {
	apiKey := os.Getenv("YANDEX_API_KEY")
	folderID := os.Getenv("YANDEX_FOLDER_ID")

	url := "https://llm.api.cloud.yandex.net/foundationModels/v1/completion"

	requestBody := GPTRequest{
		ModelUri: fmt.Sprintf("gpt://%s/yandexgpt-lite", folderID),
		CompletionOptions: CompletionOptions{
			Stream:      false,
			Temperature: 0.7,
			MaxTokens:   maxTokens,
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
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Ошибка загрузки .env файла")
	}

	InitDatabase()

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
			if update.Message.Command() == "start" {
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "⏳ Генерирую темы, подожди секунду...")
				bot.Send(msg)

				topicsText, err := askYandexGPT("Сгенерируй 10 интересных, разнообразных тем для историй. Ответ выдай в виде пронумерованного списка из 2 или 3 слов.", 300)
				if err != nil {
					bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "❌ Ошибка генерации тем: "+err.Error()))
					continue
				}

				lines := strings.Split(topicsText, "\n")
				topics := make([]string, 0, 10)
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

				sessions[update.Message.Chat.ID] = &Session{Topics: topics, SelectedTopics: []int{}, ChosenIndex: -1}

				rows := [][]tgbotapi.InlineKeyboardButton{}
				for i, topic := range topics {
					btn := tgbotapi.NewInlineKeyboardButtonData(topic, fmt.Sprintf("topic_%d", i))
					rows = append(rows, tgbotapi.NewInlineKeyboardRow(btn))
				}
				rows = append(rows, tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData("🔁 Сгенерировать заново", "restart")))

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
				topicsText, err := askYandexGPT("Сгенерируй 10 интересных, разнообразных тем для историй. Ответ выдай в виде пронумерованного списка из 2 или 3 слов.", 300)
				if err != nil {
					bot.Send(tgbotapi.NewMessage(chatID, "❌ Ошибка генерации тем: "+err.Error()))
					continue
				}

				lines := strings.Split(topicsText, "\n")
				topics := make([]string, 0, 10)
				for _, line := range lines {
					line = strings.TrimSpace(line)
					if i := strings.Index(line, "."); i != -1 {
						line = strings.TrimSpace(line[i+1:])
					}
					if line != "" {
						topics = append(topics, line)
					}
				}
				if len(topics) > 10 {
					topics = topics[:10]
				}

				sessions[chatID] = &Session{Topics: topics, SelectedTopics: []int{}, ChosenIndex: -1}
				rows := [][]tgbotapi.InlineKeyboardButton{}
				for i, topic := range topics {
					btn := tgbotapi.NewInlineKeyboardButtonData(topic, fmt.Sprintf("topic_%d", i))
					rows = append(rows, tgbotapi.NewInlineKeyboardRow(btn))
				}
				rows = append(rows, tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData("🔁 Сгенерировать заново", "restart")))

				keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)
				msg := tgbotapi.NewMessage(chatID, "Выбери 3 интересных темы:")
				msg.ReplyMarkup = keyboard
				bot.Send(msg)
				continue
			}

			if strings.HasPrefix(data, "topic_") {
				var index int
				fmt.Sscanf(data, "topic_%d", &index)
				sess := sessions[chatID]
				for _, v := range sess.SelectedTopics {
					if v == index {
						bot.Send(tgbotapi.NewMessage(chatID, "⚠️ Эта тема уже выбрана."))
						continue
					}
				}
				sess.SelectedTopics = append(sess.SelectedTopics, index)
				if len(sess.SelectedTopics) < 3 {
					topic := sess.Topics[index]
					bot.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("✅ Тема добавлена: %s (%d из 3)", topic, len(sess.SelectedTopics))))
					continue
				}

				rows := [][]tgbotapi.InlineKeyboardButton{}
				for _, i := range sess.SelectedTopics {
					text := sess.Topics[i]
					btn := tgbotapi.NewInlineKeyboardButtonData(text, fmt.Sprintf("chosen_%d", i))
					rows = append(rows, tgbotapi.NewInlineKeyboardRow(btn))
				}
				keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)

				msg := tgbotapi.NewMessage(chatID, "Теперь выбери одну тему, по которой хочешь услышать историю:")
				msg.ReplyMarkup = keyboard
				bot.Send(msg)
				continue
			}

			if strings.HasPrefix(data, "chosen_") {
				var index int
				fmt.Sscanf(data, "chosen_%d", &index)
				sess := sessions[chatID]
				sess.ChosenIndex = index
				topic := sess.Topics[index]
				storyPrompt := fmt.Sprintf("Придумай интересную, подробную историю на тему: %s. Не превышай 10000 токенов.", topic)
				story, err := askYandexGPT(storyPrompt, 800)
				if err != nil {
					bot.Send(tgbotapi.NewMessage(chatID, "Ошибка генерации истории: "+err.Error()))
					continue
				}
				bot.Send(tgbotapi.NewMessage(chatID, "📚 История по теме \""+topic+"\":\n\n"+story))
				continue
			}
		}
	}
}
