package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"

	"Go_project/internal/domain"
	"Go_project/internal/repository"
)

func main() {
	mongoURI := os.Getenv("MONGO_URI")
	if mongoURI == "" {
		mongoURI = "mongodb://localhost:27017"
	}

	mongoDBName := os.Getenv("MONGO_DB")
	if mongoDBName == "" {
		mongoDBName = "app_db"
	}

	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	// 1. Инициализация стораджа
	storage, err := repository.NewStorage(mongoURI, mongoDBName, redisAddr)
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}

	userRepo := repository.NewMongoUserRepository(storage.MongoDB)
	cacheRepo := repository.NewRedisCacheRepository(storage.RedisClient)

	// 2. Хэндлер создания пользователя (MongoDB)
	http.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			var u domain.User
			if err := json.NewDecoder(r.Body).Decode(&u); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			if err := userRepo.Create(&u); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(u)
			return
		}
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	})

	// 3. Хэндлер со счетчиком запросов (Redis)
	http.HandleFunc("/hits", func(w http.ResponseWriter, r *http.Request) {
		count, err := cacheRepo.IncrementCounter("api_hits")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message":    "Page view counted",
			"total_hits": count,
		})
	})

	log.Println("Server running on port 8080...")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}