package repository

import (
	"context"
	"log"
	"time"
	
	"github.com/go-redis/redis/v8"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type Storage struct{
	MongoClient *mongo.Client
	MongoDB *mongo.Database
	RedisClient *redis.Client
}

func NewStorage(mongoURI, dbName, redisAddr string)(*Storage, error){
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	clientOptions := options.Client().ApplyURI(mongoURI)

	mongoClient, err := mongo.Connect(clientOptions)
	if err !=nil {
		return nil, err
	}

	if err := mongoClient.Ping(ctx, nil); err != nil{
		return nil, err
	}
	log.Println("Succesfully connected to MongoDB")

	rdb := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})

	if err := rdb.Ping(ctx).Err(); err != nil{
		return nil,err
	}
	log.Println("Seccesfully connected to redis")

	return &Storage{
		MongoClient:  mongoClient,
		MongoDB: mongoClient.Database(dbName),
		RedisClient: rdb,
	},nil 
}