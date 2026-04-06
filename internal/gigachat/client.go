package gigachat

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"log"
	"os"
	"strings"
	"time"
)

// Client — минимальный клиент GigaChat, используемый в скелете.
type Client interface {
	Summarize(ctx context.Context, text string) (string, error)
	Chat(ctx context.Context, question string) (string, error)
}

type clientGiga struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

func NewClient() Client {
	return &clientGiga{
		baseURL:    os.Getenv("GIGA_BASE_URL"),
		apiKey:     os.Getenv("GIGA_API_KEY"),
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

func (m *clientGiga) Summarize(ctx context.Context, text string) (string, error) {

	systemContent := "Ты — секретарь. Твоя роль суммаризировать стенограмму встречи. Выжимка должна быть краткой, не более 3-4 предложений. Не добавляй ничего, что не было сказано в стенограмме. Нужно добавить название встречи, если оно есть, если его нет то придумай сам."

	replace := strings.ReplaceAll(text, "\n", "- ")

	body, err := m.sendRequestToGiga(systemContent, replace)
	if err != nil {
		return "", err
	}
	return m.parsed(body)
}

func (m *clientGiga) sendRequestToGiga(systemPromt, userContent string) ([]byte, error) {

	bodyStr := fmt.Sprintf(`{
		"model": "GigaChat",
		"messages": [
			{"role": "system", "content": "%s"},
			{"role": "user", "content": "%s"}
		],
		"stream": false,
		"update_interval": 0
	}`, systemPromt, userContent)

	payload := strings.NewReader(bodyStr)
	req, err := http.NewRequest("POST", m.baseURL, payload)

	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", m.apiKey))

	res, err := m.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := res.Body.Close(); err != nil {
			log.Printf("failed to close http response body: %v", err)
		}
	}()

	respBody, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	return respBody, nil
}

func (m *clientGiga) Chat(ctx context.Context, question string) (string, error) {

	systemContent := "Ты — ИИ ассистент. Твоя роль — помогать пользователю отвечая на его вопросы. Ответ укладывай в 2-3 предложения, если вопрос не требует развернутого ответа. Если вопрос требует развернутого ответа, то дай его, но не добавляй ничего лишнего, не выдумывай факты, не добавляй ничего, что не было сказано в вопросе."
	body, err := m.sendRequestToGiga(systemContent, question)
	if err != nil {
		return "", err
	}
	return m.parsed(body)
}

func (m *clientGiga) parsed(body []byte) (string, error) {
	// Попытаться распарсить JSON-ответ и извлечь content из choices[0].message.content
	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
				Role    string `json:"role"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &parsed); err == nil {
		if len(parsed.Choices) > 0 {
			return parsed.Choices[0].Message.Content, nil
		}
	}

	// Фолбэк: вернуть сырый ответ если парсинг не удался или нет content
	return string(body), nil
}
