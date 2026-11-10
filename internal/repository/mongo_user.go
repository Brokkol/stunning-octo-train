package repository

import (
	"context"
	"time"

	"Go_project/internal/domain"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type MongoUserRepository struct {
	collection *mongo.Collection
}

func NewMongoUserRepository(db *mongo.Database) *MongoUserRepository {
	return &MongoUserRepository{
		collection: db.Collection("users"),
	}
}

func (r *MongoUserRepository) Create(user *domain.User) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	result, err := r.collection.InsertOne(ctx, user)
	if err != nil {
		return err
	}

	if oid, ok := result.InsertedID.(bson.ObjectID); ok {
		user.ID = oid.Hex()
	}

	return nil
}

func (r *MongoUserRepository) GetByID(id string) (*domain.User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	objID, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}

	var user domain.User
	err = r.collection.FindOne(ctx, bson.M{"_id": objID}).Decode(&user)
	if err != nil {
		return nil, err
	}

	return &user, nil
}
// Upsert: при первом входе создаёт пользователя, потом только обновляет имя, почту и время входа
func (r *MongoUserRepository) Upsert(uid, name, email string) (*domain.User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	now := time.Now().UTC()
	update := bson.M{
		"$set":         bson.M{"name": name, "email": email, "last_login_at": now},
		"$setOnInsert": bson.M{"created_at": now}, // только при создании
	}
	opts := options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)

	var user domain.User
	err := r.collection.FindOneAndUpdate(ctx, bson.M{"_id": uid}, update, opts).Decode(&user)
	if err != nil {
		return nil, err
	}
	return &user, nil
}