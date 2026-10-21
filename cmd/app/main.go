package main

import (
	"encoding/json"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"Go_project/internal/broker"
	"Go_project/internal/domain"
	"Go_project/internal/repository"
	"Go_project/internal/storage"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	db, err := repository.NewStorage(os.Getenv("MONGO_URI"), os.Getenv("MONGO_DB"), os.Getenv("REDIS_ADDR"))
	if err != nil {
		log.Fatal("MongoDB/Redis error: ", err)
	}

	s3Client, err := storage.NewS3Client()
	if err != nil {
		log.Fatal("S3 error: ", err)
	}

	rabbit, err := broker.NewRabbitMQ()
	if err != nil {
		log.Fatal("RabbitMQ error: ", err)
	}

	userRepo := repository.NewMongoUserRepository(db.MongoDB)
	imageRepo := repository.NewMongoImageRepository(db.MongoDB)
	cacheRepo := repository.NewRedisCacheRepository(db.RedisClient)

	http.HandleFunc("POST /users", func(w http.ResponseWriter, r *http.Request) {
		var user domain.User
		err := json.NewDecoder(r.Body).Decode(&user)
		if err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}

		err = userRepo.Create(&user)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		sendJSON(w, user)
	})

	http.HandleFunc("GET /users/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		cacheKey := "user:" + id

		user, err := cacheRepo.GetUser(cacheKey)
		if err == nil {
			w.Header().Set("X-Cache", "HIT")
			sendJSON(w, user)
			return
		}

		user, err = userRepo.GetByID(id)
		if err != nil {
			http.Error(w, "user not found", http.StatusNotFound)
			return
		}

		cacheRepo.SetUser(cacheKey, user)

		w.Header().Set("X-Cache", "MISS")
		sendJSON(w, user)
	})

	http.HandleFunc("GET /hits", func(w http.ResponseWriter, r *http.Request) {
		count, err := cacheRepo.IncrementCounter("api_hits")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		sendJSON(w, map[string]int64{"total_hits": count})
	})

	http.HandleFunc("POST /images", func(w http.ResponseWriter, r *http.Request) {
		file, header, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "send a file in the 'file' field", http.StatusBadRequest)
			return
		}
		defer file.Close()

		// Проверяем расширение файла
		ext := strings.ToLower(filepath.Ext(header.Filename))
		if ext != ".jpg" && ext != ".jpeg" && ext != ".png" {
			http.Error(w, "only .jpg and .png are allowed", http.StatusBadRequest)
			return
		}

		// Придумываем id и имя файла в S3
		id := bson.NewObjectID().Hex()
		image := domain.Image{
			ID:          id,
			OriginalKey: "originals/" + id + ext,
			Status:      "pending",
		}

		// Кладём файл в S3
		contentType := "image/jpeg"
		if ext == ".png" {
			contentType = "image/png"
		}
		err = s3Client.Upload(image.OriginalKey, file, contentType)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Сохраняем запись в MongoDB
		err = imageRepo.Create(&image)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Отправляем задачу через RabbitMQ
		task, _ := json.Marshal(domain.ImageTask{ImageID: id, RequestID: r.Header.Get("X-Request-ID")})
		err = rabbit.Publish(task)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		sendJSON(w, image)
	})

	http.HandleFunc("GET /images", func(w http.ResponseWriter, r *http.Request) {
		images, err := imageRepo.List()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		sendJSON(w, images)
	})

	// Узнать статус картинки
	http.HandleFunc("GET /images/{id}", func(w http.ResponseWriter, r *http.Request) {
		image, err := imageRepo.GetByID(r.PathValue("id"))
		if err != nil {
			http.Error(w, "image not found", http.StatusNotFound)
			return
		}
		sendJSON(w, image)
	})

	// Скачать оригинал
	http.HandleFunc("GET /images/{id}/original", func(w http.ResponseWriter, r *http.Request) {
		image, err := imageRepo.GetByID(r.PathValue("id"))
		if err != nil {
			http.Error(w, "image not found", http.StatusNotFound)
			return
		}
		sendFileFromS3(w, s3Client, image.OriginalKey)
	})

	// Скачать превью
	http.HandleFunc("GET /images/{id}/thumbnail", func(w http.ResponseWriter, r *http.Request) {
		image, err := imageRepo.GetByID(r.PathValue("id"))
		if err != nil {
			http.Error(w, "image not found", http.StatusNotFound)
			return
		}
		if image.Status != "done" {
			http.Error(w, "thumbnail is not ready, status: "+image.Status, http.StatusNotFound)
			return
		}
		sendFileFromS3(w, s3Client, image.ThumbnailKey)
	})
	http.Handle("GET /metrics", promhttp.Handler())
	log.Println("Server running on port 8080...")
	log.Fatal(http.ListenAndServe(":8080", Logs(http.DefaultServeMux)))
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

var httpRequests = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "http_requests_total",
	Help: "Сколько запросов обработано",
}, []string{"route", "status"})

var httpDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "http_request_duration_seconds",
	Help:    "Сколько секунд занял запрос",
	Buckets: prometheus.DefBuckets,
}, []string{"route"})

func Logs(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = bson.NewObjectID().Hex()
		}
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: 200}
		w.Header().Set("X-Request-ID", requestID)
		r.Header.Set("X-Request-ID", requestID)
		next.ServeHTTP(rec, r)
		route := r.Pattern
		if route == "" {
			route = "unmatched"
		}
		httpRequests.WithLabelValues(route, strconv.Itoa(rec.status)).Inc()
		httpDuration.WithLabelValues(route).Observe(time.Since(start).Seconds())
		slog.Info("request", "request_id", requestID, "method", r.Method, "path", r.URL.Path, "duration_ms", time.Since(start).Milliseconds(), "status", rec.status, "ip", r.Header.Get("X-Real-IP"))
	})
}

// sendJSON превращает данные в JSON и отправляет клиенту
func sendJSON(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

// sendFileFromS3 скачивает файл из S3 и сразу отдаёт его клиенту
func sendFileFromS3(w http.ResponseWriter, s3Client *storage.S3Client, key string) {
	file, err := s3Client.Download(key)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer file.Close()

	io.Copy(w, file)
}
