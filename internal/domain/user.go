package domain

import "time"

type User struct {
	ID          string    `json:"id,omitempty" bson:"_id,omitempty"`
	Name        string    `json:"name" bson:"name"`
	Email       string    `json:"email" bson:"email"`
	CreatedAt   time.Time `json:"created_at" bson:"created_at"`
	LastLoginAt time.Time `json:"last_login_at" bson:"last_login_at"`
}

type UserRepository interface {
	Create(user *User) error
	GetByID(id string) (*User, error)
}

type CacheRepository interface {
	SetUser(key string, user *User) error
	GetUser(key string) (*User, error)
	IncrementCounter(key string) (int64, error)
}