package service

import (
	"context"
	"fmt"
	"smart-daily/internal/model"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type AuthService struct{ db *gorm.DB }

func NewAuthService(db *gorm.DB) *AuthService { return &AuthService{db: db} }

func (s *AuthService) Login(ctx context.Context, username, password string) (*model.Member, error) {
	var m model.Member
	if err := s.db.WithContext(ctx).Where("username = ?", username).First(&m).Error; err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}
	if bcrypt.CompareHashAndPassword([]byte(m.Password), []byte(password)) != nil {
		return nil, fmt.Errorf("wrong password")
	}
	return &m, nil
}
