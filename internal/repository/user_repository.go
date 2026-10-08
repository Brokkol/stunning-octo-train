package repository

import "errors"

type User struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Email  string `json:"email"`
}

type Stats struct {
	Status  string `json:"status"`
	Uptime  string `json:"uptime"`
	Requests int    `json:"requests"`
}

type UserRepository struct{}

func NewUserRepository() *UserRepository {
	return &UserRepository{}
}

func (r *UserRepository) GetByID(id string) (*User, error) {
	if id == "1" {
		return &User{
			ID:     "1",
			Name:   "Stas",
			Email:  "email@test.com",
		}, nil
	}
	return nil, errors.New("user not found")
}

func (r *UserRepository) GetStats() *Stats {
	return &Stats{
		Status:   "running",
		Uptime:   "7m",
		Requests: 100,
	}
}