package service

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"magaz/internal/models"
	"magaz/internal/repository"
	"magaz/internal/validation"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrEmailTaken     = errors.New("email уже зарегистрирован")
	ErrInvalidCreds   = errors.New("неверный email или пароль")
	ErrInvalidToken   = errors.New("ссылка недействительна или устарела")
	ErrWeakPassword   = errors.New("пароль должен быть не менее 8 символов")
)

const bcryptCost = 12

type AuthService struct {
	users *repository.UserRepository
	guard *BruteGuard
}

func NewAuthService(users *repository.UserRepository) *AuthService {
	return &AuthService{
		users: users,
		guard: newBruteGuard(),
	}
}

// dummyHash is pre-computed in init() and used to prevent timing attacks:
// when a user does not exist, we still run bcrypt.Compare to match the
// response time of a real failed login — preventing user-existence enumeration.
var dummyHash []byte

func init() {
	var err error
	dummyHash, err = bcrypt.GenerateFromPassword([]byte("timing-protection-dummy-pw!"), 12)
	if err != nil {
		// This is a startup failure — cannot protect against timing attacks without it.
		panic("auth: failed to generate brute-force dummy hash: " + err.Error())
	}
}

type RegisterInput struct {
	Email    string `validate:"required,email,max=255"`
	Name     string `validate:"required,min=2,max=100"`
	Password string `validate:"required,min=8,max=72"`
}

func (s *AuthService) Register(in RegisterInput) (*models.User, error) {
	if len(in.Password) < 8 {
		return nil, ErrWeakPassword
	}

	// Validate name: not an email, valid characters only
	if err := validation.ValidateName(in.Name); err != nil {
		return nil, err
	}

	_, err := s.users.FindByEmail(in.Email)
	if err == nil {
		return nil, ErrEmailTaken
	}
	if !errors.Is(err, repository.ErrNotFound) {
		return nil, fmt.Errorf("check email: %w", err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcryptCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	u := &models.User{
		Email:        in.Email,
		Name:         in.Name,
		PasswordHash: string(hash),
		Role:         models.RoleUser,
	}
	if err := s.users.Create(u); err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	return u, nil
}

func (s *AuthService) Login(email, password string) (*models.User, error) {
	// 1. Check if account is currently locked before any work
	if err := s.guard.Check(email); err != nil {
		return nil, err
	}

	// 2. Fetch user record
	u, err := s.users.FindByEmail(email)
	if errors.Is(err, repository.ErrNotFound) {
		// Compare against dummy hash so response time is identical to a real
		// failed attempt — prevents user-existence enumeration via timing.
		_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(password))
		s.guard.RecordFailure(email)
		return nil, ErrInvalidCreds
	}
	if err != nil {
		return nil, err
	}

	// 3. Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		s.guard.RecordFailure(email)

		// Show warning when account is now locked
		if lockErr := s.guard.Check(email); lockErr != nil {
			return nil, lockErr
		}

		// Show remaining attempts when low
		left := s.guard.AttemptsLeft(email)
		if left > 0 && left <= 3 {
			return nil, fmt.Errorf("неверный пароль — осталось попыток до блокировки: %d", left)
		}
		return nil, ErrInvalidCreds
	}

	// 4. Success — clear brute-force counter
	s.guard.RecordSuccess(email)
	return u, nil
}

func (s *AuthService) ForgotPassword(email string) error {
	u, err := s.users.FindByEmail(email)
	if errors.Is(err, repository.ErrNotFound) {
		// Don't reveal whether email exists
		return nil
	}
	if err != nil {
		return err
	}

	token, err := generateToken(32)
	if err != nil {
		return err
	}
	expires := time.Now().Add(time.Hour)

	if err := s.users.SetResetToken(u.ID, token, expires); err != nil {
		return err
	}

	// ⚠️  Production: send link via email (SMTP) — never log the token!
	// In development: run the following SQL to get the token:
	//   SELECT reset_token FROM users WHERE email = '...';
	slog.Info("password reset requested", "user_id", u.ID, "email", email)
	return nil
}

func (s *AuthService) ResetPassword(token, newPassword string) error {
	if len(newPassword) < 8 {
		return ErrWeakPassword
	}

	u, err := s.users.FindByResetToken(token)
	if errors.Is(err, repository.ErrNotFound) {
		return ErrInvalidToken
	}
	if err != nil {
		return err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcryptCost)
	if err != nil {
		return err
	}
	return s.users.UpdatePassword(u.ID, string(hash))
}

func (s *AuthService) GetByID(id int64) (*models.User, error) {
	return s.users.FindByID(id)
}

func (s *AuthService) UpdateName(id int64, name string) error {
	if err := validation.ValidateName(name); err != nil {
		return err
	}
	return s.users.UpdateName(id, name)
}

func generateToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
