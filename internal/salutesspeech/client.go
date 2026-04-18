package salutesspeech

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

// SaluteSpeechClient реализует интерфейс Client для обращения к REST API SaluteSpeech (синхронный endpoint распознавания).
type SaluteSpeechClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// Client определяет интерфейс клиента SaluteSpeech, используемый в приложении.
type Client interface {
	Recognize(ctx context.Context, data []byte, mime string) (*Result, error)
	RecognizeAsync(ctx context.Context, fileRef string) (*Result, error)
}

// Result содержит текст распознанной речи.
type Result struct {
	Text string
}

// NewRealClient создаёт RealClient, читая параметры из переменных окружения или из явных параметров.
func NewRealClientFromEnv() *SaluteSpeechClient {

	return &SaluteSpeechClient{
		baseURL:    os.Getenv("SALUTE_BASE_URL"),
		apiKey:     os.Getenv("SALUTE_API_KEY"),
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

// Recognize отправляет сырые байты аудио в синхронный endpoint SaluteSpeech для распознавания.
func (r *SaluteSpeechClient) Recognize(ctx context.Context, data []byte, mime string) (*Result, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty audio data")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.baseURL+"/speech:recognize", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", mime)
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", r.apiKey))

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("failed to close salutespeech response body: %v", err)
		}
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("salutespeech error: status=%d body=%s", resp.StatusCode, string(body))
	}

	// Ожидается, что ответ содержит JSON с полем `result` — массивом строк.
	var parsed struct {
		Result []string `json:"result"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse salutespeech response: %w", err)
	}

	text := ""
	for i, s := range parsed.Result {
		if i > 0 {
			text += "\n"
		}
		text += s
	}

	return &Result{Text: text}, nil
}

// RecognizeAsync не реализован в этом клиенте (заглушка).
func (r *SaluteSpeechClient) RecognizeAsync(ctx context.Context, fileRef string) (*Result, error) {
	return nil, fmt.Errorf("RecognizeAsync not implemented for SaluteSpeechClient; use Recognize with raw bytes")
}
