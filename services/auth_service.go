package services

import (
	"database/sql"
	"fmt"
	"queueflow/middleware"
	"queueflow/models"
	"queueflow/repositories"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	userRepo  *repositories.UserRepository
	jwtSecret string
}

func NewAuthService(userRepo *repositories.UserRepository, jwtSecret string) *AuthService {
	return &AuthService{
		userRepo:  userRepo,
		jwtSecret: jwtSecret,
	}
}

func (s *AuthService) Login(req models.LoginRequest) (*models.LoginResponse, error) {
	// Get user by username
	user, err := s.userRepo.GetByUsername(req.Username)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("invalid username or password")
		}
		return nil, err
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, fmt.Errorf("invalid username or password")
	}

	// Generate JWT token
	claims := middleware.JWTClaims{
		UserID:   user.ID,
		Username: user.Username,
		Role:     user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(s.jwtSecret))
	if err != nil {
		return nil, err
	}

	return &models.LoginResponse{
		Token:    tokenString,
		UserID:   user.ID,
		Username: user.Username,
		Role:     user.Role,
	}, nil
}

func (s *AuthService) Register(req models.RegisterRequest) (*models.LoginResponse, error) {
	// Validate input
	if req.Username == "" || req.Email == "" || req.Password == "" {
		return nil, fmt.Errorf("username, email, and password are required")
	}

	// Check if username already exists
	existingUser, err := s.userRepo.GetByUsername(req.Username)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	if existingUser != nil {
		return nil, fmt.Errorf("username already exists")
	}

	// Check if email already exists
	existingEmail, err := s.userRepo.GetByEmail(req.Email)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	if existingEmail != nil {
		return nil, fmt.Errorf("email already registered")
	}

	// Hash password
	passwordHash, err := HashPassword(req.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password")
	}

	// Create user with default role 'user'
	user, err := s.userRepo.Create(req.Username, req.Email, passwordHash, "user")
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Generate JWT token
	claims := middleware.JWTClaims{
		UserID:   user.ID,
		Username: user.Username,
		Role:     user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(s.jwtSecret))
	if err != nil {
		return nil, err
	}

	return &models.LoginResponse{
		Token:    tokenString,
		UserID:   user.ID,
		Username: user.Username,
		Role:     user.Role,
	}, nil
}

// UpdateFCMToken updates the FCM token for a user
func (s *AuthService) UpdateFCMToken(userID int, fcmToken string) error {
	return s.userRepo.UpdateFCMToken(userID, fcmToken)
}

// HashPassword creates a bcrypt hash of the password
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}
