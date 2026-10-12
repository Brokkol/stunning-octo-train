package repository

import (
	"context"

	"Go_project/internal/domain"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type MongoImageRepository struct {
	collection *mongo.Collection
}

func NewMongoImageRepository(db *mongo.Database) *MongoImageRepository {
	return &MongoImageRepository{
		collection: db.Collection("images"),
	}
}

// Create сохраняет новую запись о картинке
func (r *MongoImageRepository) Create(img *domain.Image) error {
	_, err := r.collection.InsertOne(context.Background(), img)
	return err
}

// GetByID ищет картинку по id
func (r *MongoImageRepository) GetByID(id string) (*domain.Image, error) {
	var img domain.Image
	err := r.collection.FindOne(context.Background(), bson.M{"_id": id}).Decode(&img)
	if err != nil {
		return nil, err
	}
	return &img, nil
}

// Update перезаписывает запись целиком (воркер вызывает это, когда поменял статус)
func (r *MongoImageRepository) Update(img *domain.Image) error {
	_, err := r.collection.ReplaceOne(context.Background(), bson.M{"_id": img.ID}, img)
	return err
}

// List возвращает все картинки, новые сверху
func (r *MongoImageRepository) List() ([]domain.Image, error) {
	// Сортируем по _id в обратном порядке: в ObjectID зашито время создания
	opts := options.Find().SetSort(bson.M{"_id": -1})

	cursor, err := r.collection.Find(context.Background(), bson.M{}, opts)
	if err != nil {
		return nil, err
	}

	images := []domain.Image{} // пустой список, чтобы в JSON был [], а не null
	err = cursor.All(context.Background(), &images)
	return images, err
}
