package domain

// Image - запись о картинке в MongoDB.
// Сама картинка лежит в S3, а тут только её "адрес" (ключ) и статус.
type Image struct {
	ID           string `json:"id" bson:"_id"`
	OriginalKey  string `json:"original_key" bson:"original_key"`
	ThumbnailKey string `json:"thumbnail_key" bson:"thumbnail_key"`
	Status       string `json:"status" bson:"status"` // "pending", "done" или "failed"
}

// ImageTask - сообщение, которое API отправляет воркеру через RabbitMQ
type ImageTask struct {
	ImageID string `json:"image_id"`
}
