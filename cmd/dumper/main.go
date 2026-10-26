package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	"Go_project/internal/storage"
)

// Контейнеры, логи которых сохраняем, и за сколько минут
var containers = []string{"go_app", "go_worker"}

const dumpMinutes = 10

// HTTP-клиент, который вместо интернета ходит в сокет Docker
var docker = &http.Client{
	Transport: &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", "/var/run/docker.sock")
		},
	},
}

// readLogs возвращает текст логов контейнера за последние minutes минут
func readLogs(name string, minutes int) ([]byte, error) {
	since := time.Now().Add(-time.Duration(minutes) * time.Minute).Unix()
	url := fmt.Sprintf("http://docker/containers/%s/logs?stdout=1&stderr=1&timestamps=1&since=%d", name, since)

	resp, err := docker.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("docker answered %s", resp.Status)
	}

	var out bytes.Buffer
	header := make([]byte, 8)
	for {
		// 8 байт заголовка: последние 4 - длина следующего куска текста
		if _, err := io.ReadFull(resp.Body, header); err != nil {
			if err == io.EOF {
				break // логи закончились
			}
			return nil, err
		}
		size := binary.BigEndian.Uint32(header[4:8])
		if _, err := io.CopyN(&out, resp.Body, int64(size)); err != nil {
			return nil, err
		}
	}
	return out.Bytes(), nil
}

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	s3, err := storage.NewS3Client()
	if err != nil {
		slog.Error("s3 init failed", "error", err.Error())
		os.Exit(1)
	}

	http.HandleFunc("POST /dump", func(w http.ResponseWriter, r *http.Request) {
		stamp := time.Now().Format("2006-01-02T15-04-05")
		var keys []string

		for _, name := range containers {
			data, err := readLogs(name, dumpMinutes)

			if err != nil {
				slog.Error("read logs failed", "container", name, "error", err.Error())
				http.Error(w, "read logs failed", http.StatusInternalServerError)
				return
			}
			if len(data) == 0 {
    			slog.Info("no logs, skipping", "container", name)
    			continue
			}
			key := fmt.Sprintf("dumps/%s_%s.log", stamp, name)
			if err := s3.Upload(key, bytes.NewReader(data), "text/plain"); err != nil {
				slog.Error("upload failed", "key", key, "error", err.Error())
				http.Error(w, "upload failed", http.StatusInternalServerError)
				return
			}
			slog.Info("dump uploaded", "key", key, "bytes", len(data))
			keys = append(keys, key)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"uploaded": keys})
	})

	slog.Info("log-dumper started", "port", 8090)
	if err := http.ListenAndServe(":8090", nil); err != nil {
		slog.Error("server stopped", "error", err.Error())
		os.Exit(1)
	}
}
