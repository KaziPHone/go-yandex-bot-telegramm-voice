package gigachat

import (
	"testing"
)

func TestParsed_Success(t *testing.T) {
	m := &clientGiga{}
	body := []byte(`{"choices":[{"message":{"role":"assistant","content":"Краткая выжимка"}}]}`)
	got, err := m.parsed(body)
	if err != nil {
		t.Fatalf("parsed returned error: %v", err)
	}
	if got != "Краткая выжимка" {
		t.Fatalf("unexpected content: %s", got)
	}
}

func TestParsed_Fallback(t *testing.T) {
	m := &clientGiga{}
	body := []byte(`not a json`) // некорректный JSON — ожидается возврат сырой строки (фолбэк)
	got, err := m.parsed(body)
	if err != nil {
		t.Fatalf("parsed returned error: %v", err)
	}
	if string(body) != got {
		t.Fatalf("expected fallback raw body, got: %s", got)
	}
}
