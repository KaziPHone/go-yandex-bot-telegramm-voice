package audio

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// ToOpus конвертирует произвольные байты аудио в OGG/Opus с помощью ffmpeg.
// Возвращает конвертированные байты и MIME-тип "audio/ogg;codecs=opus".
func ToOpus(ctx context.Context, data []byte, origName string) ([]byte, string, error) {
	// Поиск ffmpeg в PATH
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		return nil, "", fmt.Errorf("ffmpeg not found in PATH: %w", err)
	}

	// Подготовка временных файлов
	inExt := filepath.Ext(origName)
	if inExt == "" {
		inExt = ".in"
	}
	inFile, err := os.CreateTemp("", "in-*"+inExt)
	if err != nil {
		return nil, "", err
	}
	defer func() {
		if err := os.Remove(inFile.Name()); err != nil {
			log.Printf("failed to remove temp input file: %v", err)
		}
	}()

	if _, err := inFile.Write(data); err != nil {
		if cerr := inFile.Close(); cerr != nil {
			log.Printf("failed to close input temp file after write error: %v", cerr)
		}
		return nil, "", err
	}
	if cerr := inFile.Close(); cerr != nil {
		log.Printf("failed to close input temp file: %v", cerr)
	}

	outFile, err := os.CreateTemp("", "out-*.ogg")
	if err != nil {
		return nil, "", err
	}
	outName := outFile.Name()
	if cerr := outFile.Close(); cerr != nil {
		log.Printf("failed to close output temp file: %v", cerr)
	}
	defer func() {
		if err := os.Remove(outName); err != nil {
			log.Printf("failed to remove temp output file: %v", err)
		}
	}()

	// Команда ffmpeg: конвертация в opus в ogg-контейнере
	// -y — перезаписать выходной файл; -hide_banner -loglevel error — тихий режим
	// Установить частоту дискретизации 16000 и моно-канал
	// Запуск с таймаутом, если контекст не содержит дедлайна
	runCtx := ctx
	var cancel func()
	if _, ok := ctx.Deadline(); !ok {
		runCtx, cancel = context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
	}
	cmd := exec.CommandContext(runCtx, ffmpeg,
		"-y", "-hide_banner", "-loglevel", "error",
		"-i", inFile.Name(),
		"-c:a", "libopus",
		"-b:a", "64k",
		"-vbr", "on",
		"-ar", "16000",
		"-ac", "1",
		outName,
	)

	if err := cmd.Run(); err != nil {
		return nil, "", fmt.Errorf("ffmpeg failed: %w", err)
	}

	// Чтение результата
	outF, err := os.Open(outName)
	if err != nil {
		return nil, "", err
	}
	defer func() {
		if err := outF.Close(); err != nil {
			log.Printf("failed to close output file: %v", err)
		}
	}()
	outData, err := io.ReadAll(outF)
	if err != nil {
		return nil, "", err
	}

	return outData, "audio/ogg;codecs=opus", nil
}
