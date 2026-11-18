package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"Go_project/internal/domain"
)

func TestThumbnailHandler_NotReadyOrForeignReturns404(t *testing.T) {
	repo := newTestImageRepo(t)
	for _, img := range []domain.Image{
		{ID: "pending-1", OwnerUID: "stas", Status: "pending", ThumbnailKey: "thumb.jpg"},
		{ID: "failed-1", OwnerUID: "stas", Status: "failed", ThumbnailKey: "thumb.jpg"},
		{ID: "done-other", OwnerUID: "other", Status: "done", ThumbnailKey: "thumb.jpg"},
	} {
		img := img
		if err := repo.Create(&img); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	// S3 передаём пустым (nil): во всех этих случаях ответ 404 приходит раньше, чем дело дойдёт до S3.
	// Если код когда-нибудь полезет в S3 раньше времени, тест упадёт с паникой.
	handler := thumbnailHandler(repo, nil)

	tests := []struct {
		name     string
		uid      string
		id       string
		wantText string // что должно быть в тексте ответа
	}{
		{"картинка ещё обрабатывается (pending)", "stas", "pending-1", "not ready"},
		{"обработка не удалась (failed)", "stas", "failed-1", "not ready"},
		{"чужая готовая картинка", "stas", "done-other", "not found"},
		{"несуществующая картинка", "stas", "no-such-id", "not found"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/images/"+tc.id+"/thumbnail", nil)
			req.SetPathValue("id", tc.id)
			req = req.WithContext(context.WithValue(req.Context(), userCtxKey, AuthUser{UID: tc.uid}))
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusNotFound {
				t.Fatalf("ожидали 404, получили %d", rec.Code)
			}
			if !strings.Contains(rec.Body.String(), tc.wantText) {
				t.Errorf("в ответе ждали %q, получили %q", tc.wantText, rec.Body.String())
			}
		})
	}
}
