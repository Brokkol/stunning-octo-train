package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"Go_project/internal/domain"
	"Go_project/internal/repository"

	"github.com/gorilla/websocket"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// ---------- WebSocket-хаб ----------

// startWSServer поднимает тестовый сервер. Кто подключился, решаем параметром ?as=<uid>
// (в настоящем приложении uid кладёт в запрос проверка токена).
func startWSServer(t *testing.T, hub *Hub) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), userCtxKey, AuthUser{UID: r.URL.Query().Get("as")})
		hub.ServeWS(w, r.WithContext(ctx))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func dialWS(t *testing.T, srv *httptest.Server, uid string) *websocket.Conn {
	t.Helper()
	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "?as=" + uid
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("не подключились к WebSocket (%s): %v", uid, err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

// waitClients ждёт, пока сервер успеет записать все соединения в хаб
// (подключение на сервере происходит чуть позже, чем Dial вернулся у клиента).
func waitClients(t *testing.T, hub *Hub, want int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		hub.mu.Lock()
		n := len(hub.clients)
		hub.mu.Unlock()
		if n == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("в хабе не появилось %d соединений", want)
}

func TestHub_BroadcastGoesOnlyToOwner(t *testing.T) {
	hub := NewHub()
	srv := startWSServer(t, hub)

	stas := dialWS(t, srv, "stas")
	other := dialWS(t, srv, "other")
	waitClients(t, hub, 2)

	hub.Broadcast("stas", []byte(`{"image_id":"1","status":"done"}`))

	// Стас сообщение получил
	stas.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := stas.ReadMessage()
	if err != nil {
		t.Fatalf("Стас не получил сообщение: %v", err)
	}
	if string(msg) != `{"image_id":"1","status":"done"}` {
		t.Errorf("пришло не то сообщение: %s", msg)
	}

	// А чужой пользователь не получил ничего: ждём недолго и ждём таймаут
	other.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	if _, msg, err := other.ReadMessage(); err == nil {
		t.Errorf("чужой пользователь получил чужое сообщение: %s", msg)
	}
}

func TestHub_DisconnectedClientIsRemoved(t *testing.T) {
	hub := NewHub()
	srv := startWSServer(t, hub)

	conn := dialWS(t, srv, "stas")
	waitClients(t, hub, 1)

	conn.Close()
	waitClients(t, hub, 0)
}

func TestServeWS_WithoutUserReturns401(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/ws", nil) // в запросе нет пользователя

	NewHub().ServeWS(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("ожидали 401, получили %d", rec.Code)
	}
}

// ---------- Logs (middleware с метриками) ----------

func counterValue(t *testing.T, c prometheus.Counter) float64 {
	t.Helper()
	m := &dto.Metric{}
	if err := c.Write(m); err != nil {
		t.Fatalf("не прочитали счётчик: %v", err)
	}
	return m.GetCounter().GetValue()
}

func TestLogs_KeepsStatusAndCountsRequest(t *testing.T) {
	// Запрос без зарегистрированного маршрута попадает в метрику с route="unmatched"
	counter := httpRequests.WithLabelValues("unmatched", "404")
	before := counterValue(t, counter)

	handler := Logs(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("GET", "/nothing", nil))

	if rec.Code != http.StatusNotFound {
		t.Errorf("статус должен дойти до клиента без изменений: ждали 404, получили %d", rec.Code)
	}
	if rec.Header().Get("X-Request-ID") == "" {
		t.Error("в ответе нет X-Request-ID")
	}
	if got := counterValue(t, counter) - before; got != 1 {
		t.Errorf("счётчик запросов должен вырасти на 1, а вырос на %v", got)
	}
}

func TestLogs_KeepsIncomingRequestID(t *testing.T) {
	handler := Logs(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	req := httptest.NewRequest("GET", "/x", nil)
	req.Header.Set("X-Request-ID", "my-id-123")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("X-Request-ID"); got != "my-id-123" {
		t.Errorf("пришедший X-Request-ID должен сохраниться, получили %q", got)
	}
}

// ---------- ownImage (проверка владельца, нужна настоящая Mongo) ----------

func newTestImageRepo(t *testing.T) *repository.MongoImageRepository {
	t.Helper()

	uri := os.Getenv("MONGO_TEST_URI")
	if uri == "" {
		t.Skip("MONGO_TEST_URI не задан, тест с базой пропущен")
	}
	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatalf("не подключились к Mongo: %v", err)
	}
	db := client.Database(fmt.Sprintf("test_%d", time.Now().UnixNano()))
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = db.Drop(ctx)
		_ = client.Disconnect(ctx)
	})
	return repository.NewMongoImageRepository(db)
}

func TestOwnImage_OnlyOwnerGetsImage(t *testing.T) {
	repo := newTestImageRepo(t)
	if err := repo.Create(&domain.Image{ID: "img-1", OwnerUID: "stas", Filename: "cat.jpg"}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// request строит запрос "от имени" uid к адресу /images/<id>
	request := func(uid, id string) *http.Request {
		r := httptest.NewRequest("GET", "/images/"+id, nil)
		r.SetPathValue("id", id)
		if uid != "" {
			r = r.WithContext(context.WithValue(r.Context(), userCtxKey, AuthUser{UID: uid}))
		}
		return r
	}

	tests := []struct {
		name   string
		req    *http.Request
		wantOK bool
	}{
		{"владелец получает свою картинку", request("stas", "img-1"), true},
		{"чужой пользователь не получает (будет 404)", request("other", "img-1"), false},
		{"несуществующая картинка", request("stas", "no-such-id"), false},
		{"запрос без пользователя", request("", "img-1"), false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			img, ok := ownImage(tc.req, repo)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, ожидали %v", ok, tc.wantOK)
			}
			if tc.wantOK && img.ID != "img-1" {
				t.Errorf("вернулась не та картинка: %+v", img)
			}
			if !tc.wantOK && img != nil {
				t.Errorf("при отказе должен вернуться nil, получили %+v", img)
			}
		})
	}
}
