package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/XRS0/reader/backend/internal/config"
	"github.com/XRS0/reader/backend/internal/database"
	"github.com/XRS0/reader/backend/migrations"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "migration error:", err)
		os.Exit(1)
	}
}
func run() (runErr error) {
	if len(os.Args) != 2 {
		return fmt.Errorf("usage: migrate up|down|status")
	}
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	maxOpen := 5
	if raw := os.Getenv("DATABASE_MAX_OPEN_CONNS"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil {
			maxOpen = n
		}
	}
	cfg := config.Database{URL: url, MaxOpenConns: maxOpen, MaxIdleConns: 2, ConnMaxLifetime: 30 * time.Minute, PingTimeout: 10 * time.Second}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	db, err := database.Open(ctx, cfg)
	if err != nil {
		return err
	}
	defer func() {
		runErr = errors.Join(runErr, db.Close())
	}()
	switch os.Args[1] {
	case "up":
		return migrations.Up(ctx, db)
	case "down":
		return migrations.Down(ctx, db)
	case "status":
		versions, err := migrations.Status(ctx, db)
		if err == nil {
			fmt.Println("applied migrations:", versions)
		}
		return err
	default:
		return fmt.Errorf("unknown migration command %q", os.Args[1])
	}
}
