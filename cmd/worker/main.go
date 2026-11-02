// Воркер - отдельная программа. Она ждёт задачи из RabbitMQ
// и для каждой картинки делает уменьшенную копию (превью).
package main

import (
	"bytes"
	"encoding/json"
	_ "github.com/gen2brain/heic"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"image"
	"image/jpeg"
	_ "image/png"
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"Go_project/internal/broker"
	"Go_project/internal/domain"
	"Go_project/internal/repository"
	"Go_project/internal/storage"

	"golang.org/x/image/draw"
)

var thumbsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "thumbnails_total",
	Help: "Сколько миниатюр обработано",
}, []string{"result"})

var thumbDuration = promauto.NewHistogram(prometheus.HistogramOpts{
	Name:    "thumbnail_duration_seconds",
	Help:    "Сколько секунд делается миниатюра",
	Buckets: prometheus.DefBuckets,
})

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		log.Fatal(http.ListenAndServe(":2112", nil))
	}()
	thumbsTotal.WithLabelValues("done").Add(0)
	thumbsTotal.WithLabelValues("failed").Add(0)
	// Подключаемся так же, как в API
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

	imageRepo := repository.NewMongoImageRepository(db.MongoDB)

	messages, err := rabbit.Consume()
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Worker is waiting for tasks...")

	// Этот цикл никогда не заканчивается: каждое новое сообщение из очереди попадает сюда
	for msg := range messages {
		var task domain.ImageTask
		json.Unmarshal(msg.Body, &task)

		slog.Info("processing", "request_id", task.RequestID, "image_id", task.ImageID)

		start := time.Now()
		err := makeThumbnail(s3Client, imageRepo, task.ImageID)
		thumbDuration.Observe(time.Since(start).Seconds())

		if err != nil {
			thumbsTotal.WithLabelValues("failed").Inc()
			slog.Error("failed", "request_id", task.RequestID, "image_id", task.ImageID, "error", err)
		} else {
			thumbsTotal.WithLabelValues("done").Inc()
			slog.Info("done", "request_id", task.RequestID, "image_id", task.ImageID)
		}

		// Говорим RabbitMQ, что задача обработана и её можно удалить из очереди
		msg.Ack(false)
	}
}

// resizeJPEG уменьшает картинку так, чтобы длинная сторона была не больше maxSide,
// и возвращает готовый JPEG. Маленькие картинки не увеличиваются.
func resizeJPEG(src image.Image, maxSide int) ([]byte, error) {
	w, h := src.Bounds().Dx(), src.Bounds().Dy()
	if w > maxSide || h > maxSide {
		if w >= h {
			h = h * maxSide / w
			w = maxSide
		} else {
			w = w * maxSide / h
			h = maxSide
		}
	}
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Over, nil)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: 85}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func makeThumbnail(s3Client *storage.S3Client, imageRepo *repository.MongoImageRepository, imageID string) error {
	// 1. Находим запись о картинке в MongoDB
	img, err := imageRepo.GetByID(imageID)
	if err != nil {
		return err
	}

	// 2. Скачиваем оригинал из S3
	file, err := s3Client.Download(img.OriginalKey)
	if err != nil {
		return err
	}
	defer file.Close()

	// 3. Читаем файл как картинку
	original, _, err := image.Decode(file)
	if err != nil {
		img.Status = "failed"
		imageRepo.Update(img)
		return err
	}

		// 4. Делаем две JPEG-версии: превью (300) и для просмотра (1600)
	thumbBytes, err := resizeJPEG(original, 300)
	if err != nil {
		return err
	}
	viewBytes, err := resizeJPEG(original, 1600)
	if err != nil {
		return err
	}

	// 5. Кладём обе в S3
	img.ThumbnailKey = "thumbnails/" + img.ID + ".jpg"
	err = s3Client.Upload(img.ThumbnailKey, bytes.NewReader(thumbBytes), "image/jpeg")
	if err != nil {
		return err
	}
	img.ViewKey = "views/" + img.ID + ".jpg"
	err = s3Client.Upload(img.ViewKey, bytes.NewReader(viewBytes), "image/jpeg")
	if err != nil {
		return err
	}

	// 6. Обновляем статус в MongoDB
	img.Status = "done"
	return imageRepo.Update(img)
}
