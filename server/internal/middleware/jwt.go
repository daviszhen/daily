package middleware

import (
	"crypto/rand"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// JWTSecret is regenerated on each server start, invalidating all existing tokens.
var JWTSecret = generateSecret()

const tokenTTL = 36 * time.Hour // daily users stay logged in; skip a day → re-login

func TokenTTL() time.Duration { return tokenTTL }

func generateSecret() []byte {
	b := make([]byte, 32)
	rand.Read(b)
	return b
}

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
		if admin, ok := claims["is_admin"].(bool); ok && admin {
			c.Set("is_admin", true)
		}

		// Auto-renew when less than 12 hours remaining
		if exp, ok := claims["exp"].(float64); ok {
			if time.Until(time.Unix(int64(exp), 0)) < 12*time.Hour {
				newToken, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
					"uid":      claims["uid"],
					"name":     claims["name"],
					"is_admin": claims["is_admin"],
					"exp":      time.Now().Add(tokenTTL).Unix(),
				}).SignedString(JWTSecret)
				c.Header("X-New-Token", newToken)
			}
		}

		c.Next()
	}
}

func AdminOnly() gin.HandlerFunc {
	return func(c *gin.Context) {
		if admin, _ := c.Get("is_admin"); admin != true {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "admin only"})
			return
		}
		c.Next()
	}
}

func IsAdmin(c *gin.Context) bool {
	admin, _ := c.Get("is_admin")
	return admin == true
}
