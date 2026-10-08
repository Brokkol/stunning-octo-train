package domain

type User struct {
	ID    string `json:"id,omitempty" bson:"_id,omitempty"`
	Name  string `json:"name" bson:"name"`
	Email string `json:"email" bson:"email"`
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