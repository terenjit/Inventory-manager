package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
	"inventory-manager/internal/auth"
	"inventory-manager/internal/config"
	"inventory-manager/internal/db"
	"inventory-manager/internal/store"
)

func main() {
	_ = godotenv.Load()
	cfg := config.Load()

	database, err := db.New(cfg.DBDSN)
	if err != nil {
		log.Fatalf("connect db: %v", err)
	}
	defer database.Close()

	ctx := context.Background()
	storeRepo := store.NewRepository(database)
	userRepo := auth.NewRepository(database)

	// Seed stores
	seeds := []store.Store{
		{Code: "SWT", Name: "Sumbawa Textile"},
		{Code: "STT", Name: "Sumbawa Textile Taliwang"},
	}
	for _, s := range seeds {
		id, err := storeRepo.Create(ctx, &s)
		if err != nil {
			fmt.Printf("store %q already exists or error: %v\n", s.Name, err)
			continue
		}
		fmt.Printf("created store: %s (id=%d)\n", s.Name, id)
	}

	// Seed admin user
	adminPassword := getEnv("SEED_ADMIN_PASSWORD", "admin123")
	hash, err := bcrypt.GenerateFromPassword([]byte(adminPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("hash password: %v", err)
	}
	id, err := userRepo.Create(ctx, &auth.User{
		Username:     "admin",
		FullName:     "Administrator",
		PasswordHash: string(hash),
		Role:         "admin",
	})
	if err != nil {
		fmt.Printf("admin user already exists or error: %v\n", err)
		return
	}
	fmt.Printf("created admin: username=admin password=%s (id=%d)\n", adminPassword, id)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
