package service

import (
	"context"
	"fmt"
	"smart-daily/internal/model"
	"smart-daily/internal/repository"

	"golang.org/x/crypto/bcrypt"
)

type AuthService struct{ repo *repository.MemberRepo }

func NewAuthService(repo *repository.MemberRepo) *AuthService { return &AuthService{repo: repo} }

func (s *AuthService) Login(ctx context.Context, username, password string) (*model.Member, error) {
	m, err := s.repo.FindByUsername(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}
	if bcrypt.CompareHashAndPassword([]byte(m.Password), []byte(password)) != nil {
		return nil, fmt.Errorf("wrong password")
	}
	return m, nil
}
