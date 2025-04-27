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
	"time"

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
	Topics           []string
	SelectedTopics   []int
	ChosenIndex      int
	CurrentUser      string // текущий пользователь 1 или 2
	CurrentQuestion  int    // индекс текущего вопроса
	FirstUserAnswer  []string
	SecondUserAnswer []string
	IsFinished       bool     // оба прошли все вопросы
	RefinedTopics    []string // темы по мэтчу
}

var sessions = make(map[int64]*Session)

// var GlobalTopics = []string{
// 	"животные", "автомобили", "еда", "программирование", "музыка",
// 	"кино", "спорт", "космос", "мода", "путешествия",
// }

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

func generatePopularTopics() ([]string, error) {
	prompt := "Сгенерируй 10 самых популярных тем, которые интересны мужчинам и женщинам в возрасте от 20 до 40 лет. Темы должны быть универсальными, современными и разнообразными. Ответ выдай в виде пронумерованного списка, каждая тема — 1-2 слова, без точки в конце. Например: 1. Путешествия"
	response, err := askYandexGPT(prompt, 400)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(response, "\n")
	topics := []string{}
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
	return topics, nil
}

func sendInterestQuestion(bot *tgbotapi.BotAPI, chatID int64) {
	session := sessions[chatID]
	if session.CurrentQuestion >= len(session.Topics) {
		if session.CurrentUser == "first" { // первый закончил опрос
			bot.Send(tgbotapi.NewMessage(chatID, "Первый участник завершил опрос. Напиши /next, чтобы продолжить со второй половинкой!"))

		} else if session.CurrentUser == "second" {
			session.IsFinished = true
			common := getCommonInterests(session.FirstUserAnswer, session.SecondUserAnswer)

			if len(common) == 0 {
				bot.Send(tgbotapi.NewMessage(chatID, "У вас нет общих интересов 😢"))
			} else {
				bot.Send(tgbotapi.NewMessage(chatID, "У вас есть общие интересы: "+strings.Join(common, ", ")))
				bot.Send(tgbotapi.NewMessage(chatID, "Теперь я могу предложить вам интересные истории на общие темы."))

				// Запрашиваем уточнённые темы у GPT
				prompt := fmt.Sprintf(`Для каждой из тем: %s — придумай по 3 интересные подтемы(название 1-2 слова), включая популярные и редкие. Строго выведи списком в формате:
				<основная тема>: подтема1, подтема2, подтема3
				Никаких нумераций, пояснений и лишнего текста`, strings.Join(common, ", "))
				refined, err := askYandexGPT(prompt, 400)
				if err != nil {
					bot.Send(tgbotapi.NewMessage(chatID, " Ошибка при уточнении тем: "+err.Error()))
					return
				}

				// Парсим подтемы
				fmt.Println("GPT ответ по уточнённым темам:\n" + refined)

				// Парсим уточнённые темы
				lines := strings.Split(refined, "\n")
				session.RefinedTopics = []string{}
				for _, line := range lines {
					line = strings.TrimSpace(line)
					if line == "" {
						continue
					}

					// Удалим звёздочки и "подтема1:" / "подтема2:"
					line = strings.TrimPrefix(line, "*")
					if i := strings.Index(line, ":"); i != -1 {
						line = line[i+1:]
					}

					// Убираем скобки и всё внутри них (например: "полиамория (открытые отношения)" → "полиамория")
					if idx := strings.Index(line, "("); idx != -1 {
						line = line[:idx]
					}

					// Убираем точки, запятые и пробелы
					line = strings.TrimSpace(line)
					line = strings.Trim(line, ".,; ")

					if line != "" {
						session.RefinedTopics = append(session.RefinedTopics, line)
					}
				}

				//  Проверяем
				if len(session.RefinedTopics) == 0 {
					bot.Send(tgbotapi.NewMessage(chatID, " Не удалось получить уточнённые темы."))
					return
				}

				//  кнопки
				rows := [][]tgbotapi.InlineKeyboardButton{}
				for i, t := range session.RefinedTopics {
					btn := tgbotapi.NewInlineKeyboardButtonData(t, fmt.Sprintf("refined_%d", i))
					rows = append(rows, tgbotapi.NewInlineKeyboardRow(btn))
				}
				keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)
				msg := tgbotapi.NewMessage(chatID, "Выберите одну тему для генерации истории:")
				msg.ReplyMarkup = keyboard
				bot.Send(msg)
			}

		}
		return
	}

	question := fmt.Sprintf("Тебе нравится %s?", session.Topics[session.CurrentQuestion])
	yesBtn := tgbotapi.NewInlineKeyboardButtonData("Да", "yes")
	noBtn := tgbotapi.NewInlineKeyboardButtonData("Нет", "no")
	row := tgbotapi.NewInlineKeyboardRow(yesBtn, noBtn)
	keyboard := tgbotapi.NewInlineKeyboardMarkup(row)

	msg := tgbotapi.NewMessage(chatID, question)
	msg.ReplyMarkup = keyboard
	bot.Send(msg)
}

func getCommonInterests(first, second []string) []string {
	common := []string{}
	set := make(map[string]bool)

	for _, t := range first {
		set[t] = true
	}
	for _, t := range second {
		if set[t] {
			common = append(common, t)
		}
	}

	return common
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

		if update.CallbackQuery != nil {

			callback := update.CallbackQuery
			data := callback.Data
			chatID := callback.Message.Chat.ID

			ack := tgbotapi.NewCallback(callback.ID, "")
			bot.Request(ack)

			if data == "yes" || data == "no" {
				session := sessions[chatID]
				topic := session.Topics[session.CurrentQuestion]

				if session.CurrentUser == "first" {
					if data == "yes" {
						session.FirstUserAnswer = append(session.FirstUserAnswer, topic)
					}
				} else if session.CurrentUser == "second" {
					if data == "yes" {
						session.SecondUserAnswer = append(session.SecondUserAnswer, topic)
					}
				}

				session.CurrentQuestion++
				sendInterestQuestion(bot, chatID)
				continue
			}

			if data == "restart" {
				topicsText, err := askYandexGPT("Сгенерируй 10 интересных, разнообразных тем для историй, основанных на реальных фактах. Ответ выдай в виде пронумерованного списка из 2 или 3 слов.", 300)
				if err != nil {
					bot.Send(tgbotapi.NewMessage(chatID, " Ошибка генерации тем: "+err.Error()))
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
						bot.Send(tgbotapi.NewMessage(chatID, " Эта тема уже выбрана."))
						continue
					}
				}
				sess.SelectedTopics = append(sess.SelectedTopics, index)
				if len(sess.SelectedTopics) <= 3 {
					topic := sess.Topics[index]
					bot.Send(tgbotapi.NewMessage(chatID, fmt.Sprintf("✅ Тема добавлена: %s (%d из 3)", topic, len(sess.SelectedTopics))))
					if len(sess.SelectedTopics) <= 2 {
						continue
					}
				}

				rows := [][]tgbotapi.InlineKeyboardButton{}
				for _, i := range sess.SelectedTopics {
					text := sess.Topics[i]
					btn := tgbotapi.NewInlineKeyboardButtonData(text, fmt.Sprintf("chosen_%d", i))
					rows = append(rows, tgbotapi.NewInlineKeyboardRow(btn))
				}
				keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)

				msg := tgbotapi.NewMessage(chatID, "Теперь выбери одну тему, по которой хочешь прочитать историю:")
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
				storyPrompt := fmt.Sprintf("Напиши невероятную, но 100% правдивую историю по теме «%s», которая подтверждена научным интересным фактом, либо исследованием, либо любопытный факт. История должна: Содержать шокирующий/неожиданный поворот, подтвержденный фактами (укажи конкретные даты, имена, источники). Быть написана в стиле захватывающего научно-популярного расследования. Заканчиваться провокационным вопросом или неожиданным выводом. Вызывать желание немедленно поделиться этой информацией. Не превышай 10000 токенов.", topic)
				story, err := askYandexGPT(storyPrompt, 800)
				if err != nil {
					bot.Send(tgbotapi.NewMessage(chatID, "Ошибка генерации истории: "+err.Error()))
					continue
				}
				// Сохраняем в базу данных
				username := callback.From.UserName
				_, dbErr := DB.Exec(`
				INSERT INTO requests (username, timestamp, user_text, gpt_response, status)
				VALUES (?, ?, ?, ?, ?)
			`, username,
					time.Now().Format("2006-01-02 15:04:05"),
					topic,
					story,
					"успех",
				)

				if dbErr != nil {
					log.Println(" Ошибка сохранения в БД:", dbErr)
				}

				bot.Send(tgbotapi.NewMessage(chatID, " История по теме \""+topic+"\":\n\n"+story))
				continue
			}

			if strings.HasPrefix(data, "refined_") {
				var index int
				fmt.Sscanf(data, "refined_%d", &index)

				session := sessions[chatID]
				if index < 0 || index >= len(session.RefinedTopics) {
					bot.Send(tgbotapi.NewMessage(chatID, "Неверный выбор темы"))
					continue
				}

				selected := session.RefinedTopics[index]

				// Генерация истории
				prompt := fmt.Sprintf("Напиши невероятную, но 100% правдивую историю по теме «%s», которая подтверждена научным интересным фактом, либо исследованием, либо любопытный факт. История должна: Содержать шокирующий/неожиданный поворот, подтвержденный фактами (укажи конкретные даты, имена, источники). Быть написана в стиле захватывающего научно-популярного расследования. Заканчиваться провокационным вопросом или неожиданным выводом. Вызывать желание немедленно поделиться этой информацией. Не превышай 10000 токенов.", selected)
				story, err := askYandexGPT(prompt, 800)
				if err != nil {
					bot.Send(tgbotapi.NewMessage(chatID, "Ошибка генерации истории: "+err.Error()))
					continue
				}

				bot.Send(tgbotapi.NewMessage(chatID, " История по теме \""+selected+"\":\n\n"+story))

				// Кнопки других тем + выход
				rows := [][]tgbotapi.InlineKeyboardButton{}
				for i, t := range session.RefinedTopics {
					btn := tgbotapi.NewInlineKeyboardButtonData(t, fmt.Sprintf("refined_%d", i))
					rows = append(rows, tgbotapi.NewInlineKeyboardRow(btn))
				}
				// Добавляем кнопку выхода
				exitBtn := tgbotapi.NewInlineKeyboardButtonData(" Выйти", "exit")
				rows = append(rows, tgbotapi.NewInlineKeyboardRow(exitBtn))

				keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)
				msg := tgbotapi.NewMessage(chatID, "Хочешь получить ещё одну историю? Или нажми «Выйти», если хватит :)")
				msg.ReplyMarkup = keyboard
				bot.Send(msg)

				continue
			}
			if data == "exit" {
				delete(sessions, chatID)
				bot.Send(tgbotapi.NewMessage(chatID, "Спасибо за игру! ❤️ Чтобы начать заново, напиши /start"))
				continue
			}

		}

		if update.Message != nil && update.Message.IsCommand() {

			if update.Message.Command() == "start" {
				chatID := update.Message.Chat.ID

				topics, err := generatePopularTopics()
				if err != nil {
					bot.Send(tgbotapi.NewMessage(chatID, "Ошибка генерации популярных тем: "+err.Error()))
					return
				}

				sessions[chatID] = &Session{
					CurrentUser:     "first",
					CurrentQuestion: 0,
					Topics:          topics, // сохраняем темы в сессию
				}
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Начинаем! Отвечай на вопросы 'тебе нравится ___?'\nНажимай да или нет.")
				bot.Send(msg)

				sendInterestQuestion(bot, update.Message.Chat.ID)
				continue
			}

			if update.Message != nil && update.Message.IsCommand() && update.Message.Command() == "next" {

				session := sessions[update.Message.Chat.ID]
				session.CurrentUser = "second"
				session.CurrentQuestion = 0

				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "🔄 Теперь на те же вопросы будет отвечать второй участник!")
				bot.Send(msg)

				sendInterestQuestion(bot, update.Message.Chat.ID)
				continue
			}
		}

	}
}
