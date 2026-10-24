package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"inventory-manager/internal/apperr"
	"inventory-manager/internal/store"
)

type claims struct {
	UserID  uint   `json:"user_id"`
	Role    string `json:"role"`
	StoreID *uint  `json:"store_id,omitempty"`
	jwt.RegisteredClaims
}

type Service interface {
	Login(ctx context.Context, input LoginInput) (token string, err error)
	ValidateToken(token string) (*Actor, error)
	Me(ctx context.Context, userID uint) (*UserResponse, error)
	CreateUser(ctx context.Context, actor Actor, input CreateUserInput) (*UserResponse, error)
	UpdateUser(ctx context.Context, actor Actor, id uint, input UpdateUserInput) error
	ListUsers(ctx context.Context, actor Actor) ([]*UserResponse, error)
	DeactivateUser(ctx context.Context, actor Actor, id uint) error
}

type service struct {
	repo      Repository
	storeRepo store.Repository
	secret    []byte
}

func NewService(repo Repository, storeRepo store.Repository, jwtSecret string) Service {
	return &service{repo: repo, storeRepo: storeRepo, secret: []byte(jwtSecret)}
}

func (s *service) Login(ctx context.Context, input LoginInput) (string, error) {
	u, err := s.repo.GetByUsername(ctx, input.Username)
	if err != nil {
		return "", ErrInvalidCredentials
	}
	if !u.IsActive {
		return "", ErrUserNotActive
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(input.Password)); err != nil {
		return "", ErrInvalidCredentials
	}
	return s.generateToken(u)
}

func (s *service) ValidateToken(tokenStr string) (*Actor, error) {
	t, err := jwt.ParseWithClaims(tokenStr, &claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return s.secret, nil
	})
	if err != nil || !t.Valid {
		return nil, apperr.Unauthorized("invalid or expired token")
	}
	c, ok := t.Claims.(*claims)
	if !ok {
		return nil, apperr.Unauthorized("invalid token claims")
	}
	return &Actor{UserID: c.UserID, Role: c.Role, StoreID: c.StoreID}, nil
}

func (s *service) Me(ctx context.Context, userID uint) (*UserResponse, error) {
	u, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return u.ToResponse(), nil
}

func (s *service) CreateUser(ctx context.Context, actor Actor, input CreateUserInput) (*UserResponse, error) {
	if !actor.IsAdmin() {
		return nil, apperr.Forbidden("admin only")
	}
	if input.Role == "cashier" && input.StoreID == nil {
		return nil, ErrCashierNeedsStore
	}
	if input.StoreID != nil {
		if _, err := s.storeRepo.GetByID(ctx, *input.StoreID); err != nil {
			return nil, ErrStoreNotFound
		}
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, apperr.Internal(err)
	}
	u := &User{
		Username:     input.Username,
		FullName:     input.FullName,
		PasswordHash: string(hash),
		Role:         input.Role,
		StoreID:      input.StoreID,
	}
	id, err := s.repo.Create(ctx, u)
	if err != nil {
		return nil, err
	}
	u.ID = id
	return u.ToResponse(), nil
}

func (s *service) UpdateUser(ctx context.Context, actor Actor, id uint, input UpdateUserInput) error {
	if !actor.IsAdmin() {
		return apperr.Forbidden("admin only")
	}
	return s.repo.Update(ctx, id, input)
}

func (s *service) ListUsers(ctx context.Context, actor Actor) ([]*UserResponse, error) {
	if !actor.IsAdmin() {
		return nil, apperr.Forbidden("admin only")
	}
	users, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	resp := make([]*UserResponse, len(users))
	for i, u := range users {
		resp[i] = u.ToResponse()
	}
	return resp, nil
}

func (s *service) DeactivateUser(ctx context.Context, actor Actor, id uint) error {
	if !actor.IsAdmin() {
		return apperr.Forbidden("admin only")
	}
	f := false
	return s.repo.Update(ctx, id, UpdateUserInput{IsActive: &f})
}

func (s *service) generateToken(u *User) (string, error) {
	now := time.Now()
	c := &claims{
		UserID:  u.ID,
		Role:    u.Role,
		StoreID: u.StoreID,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(12 * time.Hour)),
		},
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, c)
	return t.SignedString(s.secret)
}
