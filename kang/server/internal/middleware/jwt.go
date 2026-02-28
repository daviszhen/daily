package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

var JWTSecret = []byte("smart-daily-secret-2026")

func JWTAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := c.GetHeader("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		token, err := jwt.Parse(auth[7:], func(t *jwt.Token) (interface{}, error) {
			return JWTSecret, nil
		})
		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}
		claims := token.Claims.(jwt.MapClaims)
		c.Set("user_id", int(claims["uid"].(float64)))
		c.Set("user_name", claims["name"].(string))

		// 剩余不到1天时自动续期
		if exp, ok := claims["exp"].(float64); ok {
			if time.Until(time.Unix(int64(exp), 0)) < 24*time.Hour {
				newToken, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
					"uid":  claims["uid"],
					"name": claims["name"],
					"exp":  time.Now().Add(7 * 24 * time.Hour).Unix(),
				}).SignedString(JWTSecret)
				c.Header("X-New-Token", newToken)
			}
		}

		c.Next()
	}
}
