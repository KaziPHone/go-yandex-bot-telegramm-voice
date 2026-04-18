package salutesspeech

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRecognize_Success(t *testing.T) {
	// httptest-сервер, имитирующий синхронный endpoint SaluteSpeech
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string][]string{"result": {"hello world"}}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := &SaluteSpeechClient{
		baseURL:    ts.URL,
		apiKey:     "test",
		httpClient: ts.Client(),
	}

	res, err := client.Recognize(context.Background(), []byte{1, 2, 3}, "audio/ogg")
	if err != nil {
		t.Fatalf("Recognize returned error: %v", err)
	}
	if res == nil || res.Text != "hello world" {
		t.Fatalf("unexpected result: %#v", res)
	}
}

func TestRecognize_EmptyData(t *testing.T) {
	client := &SaluteSpeechClient{baseURL: "http://example", apiKey: "k", httpClient: http.DefaultClient}
	if _, err := client.Recognize(context.Background(), []byte{}, "audio/ogg"); err == nil {
		t.Fatalf("expected error for empty data")
	}
}
