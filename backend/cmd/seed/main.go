package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/XRS0/reader/backend/internal/auth"
	"github.com/XRS0/reader/backend/internal/config"
	"github.com/XRS0/reader/backend/internal/database"
	"github.com/XRS0/reader/backend/internal/model"
	"github.com/XRS0/reader/backend/migrations"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "seed error:", err)
		os.Exit(1)
	}
}
func run() (runErr error) {
	environment := first("APP_ENV", "BOOKFLOW_ENV")
	if strings.EqualFold(environment, "production") {
		return fmt.Errorf("seed refuses to run in production")
	}
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	db, err := database.Open(ctx, config.Database{URL: url, MaxOpenConns: 5, MaxIdleConns: 2, ConnMaxLifetime: 30 * time.Minute, PingTimeout: 10 * time.Second})
	if err != nil {
		return err
	}
	defer func() {
		runErr = errors.Join(runErr, db.Close())
	}()
	if err = migrations.Up(ctx, db); err != nil {
		return err
	}
	email := value("SEED_USER_EMAIL", "demo@bookflow.local")
	password := value("SEED_USER_PASSWORD", "BookFlow-demo-only-2026!")
	name := value("SEED_USER_NAME", "Demo Reader")
	hasher := auth.NewPasswordHasher(64*1024, 3, 2)
	hash, err := hasher.Hash(password)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		var user model.User
		err := tx.NewSelect().Model(&user).Where("lower(email)=?", strings.ToLower(email)).Scan(ctx)
		if err == nil {
			return nil
		}
		user = model.User{ID: uuid.New(), Email: strings.ToLower(email), PasswordHash: hash, DisplayName: name, Timezone: "UTC", Locale: "en", CreatedAt: now, UpdatedAt: now}
		if _, err = tx.NewInsert().Model(&user).Exec(ctx); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO reader_preferences(user_id,theme,background_color,text_color,font_family) VALUES (?,'warm','#f7f0df','#3d352a','system')`, user.ID); err != nil {
			return err
		}
		entries := []model.DictionaryEntry{{ID: uuid.New(), UserID: user.ID, SourceLanguage: "en", TargetLanguage: "ru", OriginalWord: "book", NormalizedWord: "book", Translation: "книга", Status: "known", EncounterCount: 2, FirstSeenAt: now, LastSeenAt: now, CreatedAt: now, UpdatedAt: now}, {ID: uuid.New(), UserID: user.ID, SourceLanguage: "en", TargetLanguage: "ru", OriginalWord: "flow", NormalizedWord: "flow", Translation: "поток", Status: "learning", EncounterCount: 1, FirstSeenAt: now, LastSeenAt: now, CreatedAt: now, UpdatedAt: now}}
		_, err = tx.NewInsert().Model(&entries).Exec(ctx)
		return err
	})
}
func value(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
func first(keys ...string) string {
	for _, key := range keys {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			return v
		}
	}
	return "development"
}
