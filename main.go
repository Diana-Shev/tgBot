package main

import (
	"bytes"         // для работы с байтовыми буферами(нужно при отправке запроса)
	"encoding/json" // преобразует данные в JSON и обратно
	"fmt"           // форматирование строк
	"io/ioutil"     // читает ответы от сервера
	"log"
	"net/http" // отправляет http- запросы
	"os"       // для работы с переменными окружения (.env)
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
)

// структуры, которые описывают, как будет выглядеть JSON-запрос

type Message struct {
	Role string `json:"role"`
	Text string `json:"text"`
}

type CompletionOptions struct {
	Stream      bool    `json:"stream"`      // нужно ли получать ответ по частям, пока не используем (false)
	Temperature float64 `json:"temperature"` // творчество модели, шаг 0.3, 0,0 - строго, 1,0 хаотично
	MaxTokens   int     `json:"maxTokens"`   // макс длина ответа в токенах
}

type GPTRequest struct {
	ModelUri          string            `json:"modelUri"`          // адрес модели
	CompletionOptions CompletionOptions `json:"completionOptions"` // настройки генерации
	Messages          []Message         `json:"messages"`          // сообещния в чате
}

func askYandexGPT(userText string) (string, error) { // отправляет сообщение в GPT и получает ответ
	apiKey := os.Getenv("YANDEX_API_KEY")
	folderID := os.Getenv("YANDEX_FOLDER_ID")

	url := "https://llm.api.cloud.yandex.net/foundationModels/v1/completion" //url по которому делаем запрос GPT

	requestBody := GPTRequest{ // формируем JSON- запрос
		ModelUri: fmt.Sprintf("gpt://%s/yandexgpt-lite", folderID),
		CompletionOptions: CompletionOptions{
			Stream:      false,
			Temperature: 0.7,
			MaxTokens:   100,
		},
		Messages: []Message{
			{Role: "user", Text: userText},
		},
	}

	jsonData, err := json.Marshal(requestBody) //преобразуем структуру запроса GPTRequest в JSON (в байтах), чтобы отправить в тело запроса
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData)) //создаю POST- запрос с этим JSON-ом
	if err != nil {
		return "", err
	}

	// добавляем заголовки, как в Postman
	req.Header.Set("Authorization", "Api-Key "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-folder-id", folderID)

	client := &http.Client{}    // отправляем запрос в YGPT
	resp, err := client.Do(req) // получаем ответ
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)

	var result map[string]interface{} // преобразуем JSON-ответ от GPT в map
	err = json.Unmarshal(body, &result)
	if err != nil {
		return "", err
	}

	// Парсим ответ
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
	err := godotenv.Load() //загружаем переменные из .env
	if err != nil {
		log.Fatal("Ошибка загрузки .env файла")
	}

	InitDatabase() // подключаем БД

	botToken := os.Getenv("TELEGRAM_BOT_TOKEN") //подключаемся к tgBot API с токеном
	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Panic(err)
	}

	log.Printf(" Бот запущен: %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0) // готовим бота к приему новых сообщений из чата
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)

	for update := range updates { // запускаем бесконечный цикл, обработка каждого входящего сообщения
		if update.Message == nil { //если пришло НЕ текстовое сообщение(типо фото), пропускаем
			continue
		}

		userMessage := update.Message.Text          // получаем текст от пользователя
		replyText, err := askYandexGPT(userMessage) // отправляет его в GPT
		status := "успех"
		if err != nil {
			replyText = " Ошибка: " + err.Error()
			status = "Ошибка"
		}

		// сохраняем запрос в БД
		_, dbErr := DB.Exec(`
			INSERT INTO requests (username, timestamp, user_text, gpt_response, status)
			VALUES ($1, $2, $3, $4, $5)
		`, update.Message.From.UserName, time.Now().Format(time.RFC3339), userMessage, replyText, status)

		if dbErr != nil {
			log.Println("Ошибка записи в БД:", dbErr)
		}
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, replyText) // отправляем ответ в telegram обратно пользователю
		bot.Send(msg)
	}
}
