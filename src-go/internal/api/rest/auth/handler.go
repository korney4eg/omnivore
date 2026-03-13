package auth

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/omnivore-app/omnivore/internal/api/middleware"
	"github.com/omnivore-app/omnivore/internal/config"
	"github.com/omnivore-app/omnivore/internal/db"
	"github.com/omnivore-app/omnivore/internal/redisutil"
	"golang.org/x/crypto/bcrypt"
)

// Handler contains all auth HTTP handlers.
type Handler struct {
	cfg   *config.Config
	db    *db.DB
	redis *redisutil.RedisDataSource
}

// New creates a new auth Handler.
func New(cfg *config.Config, database *db.DB, redis *redisutil.RedisDataSource) *Handler {
	return &Handler{cfg: cfg, db: database, redis: redis}
}

// loginRequest is the JSON body for POST /api/login.
type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Login validates email/password and sets an auth cookie on success.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Email == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	user, err := findUserByEmail(r.Context(), h.db, req.Email)
	if err != nil || user == nil || user.Password == nil {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(*user.Password), []byte(req.Password)); err != nil {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	if user.Status == "DELETED" {
		writeError(w, http.StatusForbidden, "account deleted")
		return
	}

	token, err := h.issueJWT(user)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to issue token")
		return
	}

	setAuthCookie(w, token, h.isSecure())
	writeJSON(w, http.StatusOK, map[string]string{"token": token})
}

// Logout clears the auth cookie.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "auth",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.isSecure(),
		SameSite: http.SameSiteStrictMode,
	})
	w.WriteHeader(http.StatusOK)
}

// signUpRequest is the JSON body for POST /api/sign-up.
type signUpRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
	Username string `json:"username"`
}

// SignUp creates a new user account.
func (h *Handler) SignUp(w http.ResponseWriter, r *http.Request) {
	var req signUpRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Email == "" || req.Password == "" || req.Username == "" || req.Name == "" {
		writeError(w, http.StatusBadRequest, "name, email, username, and password are required")
		return
	}

	// Check for duplicate email / username
	if exists, _ := emailExists(r.Context(), h.db, req.Email); exists {
		writeError(w, http.StatusConflict, "email already in use")
		return
	}
	if exists, _ := usernameExists(r.Context(), h.db, req.Username); exists {
		writeError(w, http.StatusConflict, "username already in use")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost) // cost=10 matches TS
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	user, err := createUser(r.Context(), h.db, req.Name, req.Email, req.Username, string(hash), h.cfg.AutoVerify)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	if !h.cfg.AutoVerify {
		// TODO: enqueue confirmation email
	}

	token, err := h.issueJWT(user)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to issue token")
		return
	}

	setAuthCookie(w, token, h.isSecure())
	writeJSON(w, http.StatusCreated, map[string]string{"token": token})
}

// VerifyEmail handles GET /api/verify-email?token=<token>.
func (h *Handler) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		writeError(w, http.StatusBadRequest, "missing token")
		return
	}

	if err := verifyEmailToken(r.Context(), h.db, h.redis, token); err != nil {
		writeError(w, http.StatusBadRequest, "invalid or expired token")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "email verified"})
}

// forgotPasswordRequest is the JSON body for POST /api/forgot-password.
type forgotPasswordRequest struct {
	Email string `json:"email"`
}

// ForgotPassword sends a password reset email.
func (h *Handler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	var req forgotPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Always return 200 to avoid leaking whether the email exists.
	_ = sendPasswordResetEmail(r.Context(), h.db, h.redis, req.Email, h.cfg)
	writeJSON(w, http.StatusOK, map[string]string{"message": "if the email exists, a reset link has been sent"})
}

// resetPasswordRequest is the JSON body for POST /api/reset-password.
type resetPasswordRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

// ResetPassword consumes a reset token and updates the password.
func (h *Handler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var req resetPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Token == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "token and password are required")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	if err := resetPassword(r.Context(), h.db, h.redis, req.Token, string(hash)); err != nil {
		writeError(w, http.StatusBadRequest, "invalid or expired token")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "password updated"})
}

// changePasswordRequest is the JSON body for POST /api/change-password.
type changePasswordRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

// ChangePassword updates the password for the authenticated user.
func (h *Handler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	claims := middleware.ClaimsFromContext(r.Context())
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req changePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.CurrentPassword == "" || req.NewPassword == "" {
		writeError(w, http.StatusBadRequest, "currentPassword and newPassword are required")
		return
	}

	user, err := findUserByID(r.Context(), h.db, claims.UID)
	if err != nil || user == nil || user.Password == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(*user.Password), []byte(req.CurrentPassword)); err != nil {
		writeError(w, http.StatusUnauthorized, "current password is incorrect")
		return
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	if err := updatePassword(r.Context(), h.db, claims.UID, string(newHash)); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update password")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "password changed"})
}

// issueJWT creates a signed JWT for the given user (1 year expiry, matches TS).
func (h *Handler) issueJWT(user *userRow) (string, error) {
	claims := middleware.Claims{
		UID:      user.ID,
		UserRole: user.Role,
		Email:    user.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(365 * 24 * time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(h.cfg.JWTSecret))
}

func (h *Handler) isSecure() bool {
	return h.cfg.APIEnv != "local" && h.cfg.APIEnv != ""
}

func setAuthCookie(w http.ResponseWriter, token string, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     "auth",
		Value:    token,
		Path:     "/",
		MaxAge:   365 * 24 * 60 * 60,
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
