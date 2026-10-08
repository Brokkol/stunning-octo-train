package usecase

import (
    "Go_project/internal/repository"
)

type UserUseCase struct {
    repo *repository.UserRepository
}

func NewUserUseCase(repo *repository.UserRepository) *UserUseCase {
    return &UserUseCase{repo: repo}
}

func (uc *UserUseCase) GetUserProfile(id string) (*repository.User, error) {
    return uc.repo.GetByID(id)
}