package handler

import (
	"net/http"
	"smart-daily/internal/logger"
	"smart-daily/internal/middleware"
	"smart-daily/internal/model"
	"smart-daily/internal/service"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type AuthHandler struct{ auth *service.AuthService }

func NewAuthHandler(auth *service.AuthService) *AuthHandler { return &AuthHandler{auth: auth} }

func (h *AuthHandler) Login(c *gin.Context) {
	var req model.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	m, err := h.auth.Login(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		logger.Warn("login.failed", "username", req.Username)
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	logger.Info("login.ok", "uid", m.ID, "name", m.Name)

	token, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"uid":  m.ID,
		"name": m.Name,
		"exp":  time.Now().Add(7 * 24 * time.Hour).Unix(),
	}).SignedString(middleware.JWTSecret)

	c.JSON(http.StatusOK, model.LoginResponse{
		Token: token,
		User:  model.User{ID: m.ID, Name: m.Name, Avatar: m.Avatar, Role: m.Role},
	})
}
