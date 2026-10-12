// Воркер - отдельная программа. Она ждёт задачи из RabbitMQ
// и для каждой картинки делает уменьшенную копию (превью).
package main

import (
	"bytes"
	"encoding/json"
	"image"
	"image/jpeg"
	_ "image/png" // чтобы Go умел читать PNG
	"log"
	"os"

	"Go_project/internal/broker"
	"Go_project/internal/domain"
	"Go_project/internal/repository"
	"Go_project/internal/storage"

	"golang.org/x/image/draw"
)

func main() {
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

		log.Println("Got task for image", task.ImageID)

		err := makeThumbnail(s3Client, imageRepo, task.ImageID)
		if err != nil {
			log.Println("Failed:", err)
		} else {
			log.Println("Done:", task.ImageID)
		}

		// Говорим RabbitMQ, что задача обработана и её можно удалить из очереди
		msg.Ack(false)
	}
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

	// 4. Уменьшаем до ширины 300 пикселей (высоту считаем по пропорции)
	width := 300
	height := original.Bounds().Dy() * width / original.Bounds().Dx()
	thumbnail := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.CatmullRom.Scale(thumbnail, thumbnail.Bounds(), original, original.Bounds(), draw.Over, nil)

	// 5. Превращаем превью в JPEG
	var buf bytes.Buffer
	err = jpeg.Encode(&buf, thumbnail, nil)
	if err != nil {
		return err
	}

	// 6. Кладём превью в S3
	img.ThumbnailKey = "thumbnails/" + img.ID + ".jpg"
	err = s3Client.Upload(img.ThumbnailKey, bytes.NewReader(buf.Bytes()), "image/jpeg")
	if err != nil {
		return err
	}

	// 7. Обновляем статус в MongoDB
	img.Status = "done"
	return imageRepo.Update(img)
}
