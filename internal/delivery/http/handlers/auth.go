package handlers

import (
	"crypto/subtle"
	"net/http"
	"time"

	"github.com/go-chi/render"
	"github.com/golang-jwt/jwt/v5"
)

type LoginRequest struct {
	Password string `json:"password"`
}

type LoginResponse struct {
	Token     string `json:"token"`
	ExpiresAt int64  `json:"expires_at"`
}

// Login аутентифицирует пользователя
// @Summary Логин
// @Tags Auth
// @Accept json
// @Produce json
// @Param credentials body LoginRequest true "Учетные данные"
// @Success 200 {object} LoginResponse
// @Router /api/v1/auth/login [post]
func (h *Handlers) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := render.DecodeJSON(r.Body, &req); err != nil {
		render.JSON(w, r, map[string]interface{}{
			"error": "invalid request",
		})
		return
	}

	// Сравнение за константное время — защита от timing-атак по подбору пароля.
	if subtle.ConstantTimeCompare([]byte(req.Password), []byte(h.cfg.AdminPassword)) != 1 {
		render.Status(r, http.StatusUnauthorized)
		render.JSON(w, r, map[string]interface{}{
			"error": "invalid password",
		})
		return
	}

	// Создаем JWT токен
	expiresAt := time.Now().Add(24 * time.Hour).Unix()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"exp": expiresAt,
		"iat": time.Now().Unix(),
	})

	tokenString, err := token.SignedString([]byte(h.cfg.JWTSecret))
	if err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, map[string]interface{}{
			"error": "failed to generate token",
		})
		return
	}

	render.JSON(w, r, LoginResponse{
		Token:     tokenString,
		ExpiresAt: expiresAt,
	})
}
