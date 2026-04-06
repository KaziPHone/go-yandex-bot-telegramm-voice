package audio

import (
	"context"
	"os"
	"testing"
)

func TestToOpus_NoFFmpeg(t *testing.T) {
	// Временно очистить PATH, чтобы LookPath не нашёл ffmpeg
	old := os.Getenv("PATH")
	if err := os.Setenv("PATH", ""); err != nil {
		t.Fatalf("failed to set PATH: %v", err)
	}
	defer func() {
		if err := os.Setenv("PATH", old); err != nil {
			t.Fatalf("failed to restore PATH: %v", err)
		}
	}()

	_, _, err := ToOpus(context.Background(), []byte{0, 1, 2}, "file.mp3")
	if err == nil {
		t.Fatalf("expected error when ffmpeg not found")
	}
}
