package http

import (
    "encoding/json"
    "net/http"

    "Go_project/internal/usecase"
)

type Handler struct {
    userUC *usecase.UserUseCase
}

func NewHandler(userUC *usecase.UserUseCase) *Handler {
    return &Handler{userUC: userUC}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
    mux.HandleFunc("/health", h.HealthHandler)
    mux.HandleFunc("/api/user", h.UserHandler)
    mux.HandleFunc("/api/stats", h.StatsHandler)
}

func (h *Handler) HealthHandler(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)
    w.Write([]byte("OK"))
}

func (h *Handler) UserHandler(w http.ResponseWriter, r *http.Request) {
    user, err := h.userUC.GetUserProfile("1")
    if err != nil {
        http.Error(w, err.Error(), http.StatusNotFound)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(user)
}

func (h *Handler) StatsHandler(w http.ResponseWriter, r *http.Request) {
    stats := map[string]interface{}{
        "status":   "running",
        "uptime":   "7m",
        "requests": 100,
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(stats)
}