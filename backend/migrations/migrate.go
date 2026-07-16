package migrations

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/uptrace/bun"
)

//go:embed *.sql
var files embed.FS

type Migration struct {
	Version int64
	Name    string
	Up      string
	Down    string
}

func List() ([]Migration, error) {
	entries, err := fs.ReadDir(files, ".")
	if err != nil {
		return nil, err
	}
	byVersion := map[int64]*Migration{}
	for _, e := range entries {
		parts := strings.SplitN(e.Name(), "_", 2)
		if len(parts) != 2 {
			continue
		}
		v, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			continue
		}
		m := byVersion[v]
		if m == nil {
			m = &Migration{Version: v}
			byVersion[v] = m
		}
		body, err := files.ReadFile(e.Name())
		if err != nil {
			return nil, err
		}
		switch {
		case strings.HasSuffix(e.Name(), ".up.sql"):
			m.Up = string(body)
			m.Name = strings.TrimSuffix(parts[1], ".up.sql")
		case strings.HasSuffix(e.Name(), ".down.sql"):
			m.Down = string(body)
		}
	}
	result := make([]Migration, 0, len(byVersion))
	for _, m := range byVersion {
		if m.Up != "" {
			result = append(result, *m)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Version < result[j].Version })
	return result, nil
}

func ensure(ctx context.Context, db *bun.DB) error {
	_, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version bigint PRIMARY KEY, name text NOT NULL, applied_at timestamptz NOT NULL)`)
	return err
}

func Up(ctx context.Context, db *bun.DB) error {
	if err := ensure(ctx, db); err != nil {
		return err
	}
	list, err := List()
	if err != nil {
		return err
	}
	for _, m := range list {
		var exists bool
		if err := db.NewSelect().ColumnExpr("EXISTS(SELECT 1 FROM schema_migrations WHERE version = ?)", m.Version).Scan(ctx, &exists); err != nil {
			return err
		}
		if exists {
			continue
		}
		if err := db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
			if _, err := tx.ExecContext(ctx, m.Up); err != nil {
				return fmt.Errorf("migration %d %s: %w", m.Version, m.Name, err)
			}
			_, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations(version,name,applied_at) VALUES (?,?,?)`, m.Version, m.Name, time.Now().UTC())
			return err
		}); err != nil {
			return err
		}
	}
	return nil
}

func Down(ctx context.Context, db *bun.DB) error {
	if err := ensure(ctx, db); err != nil {
		return err
	}
	list, err := List()
	if err != nil {
		return err
	}
	var current int64
	if err := db.NewSelect().Table("schema_migrations").ColumnExpr("COALESCE(MAX(version),0)").Scan(ctx, &current); err != nil {
		return err
	}
	for i := len(list) - 1; i >= 0; i-- {
		m := list[i]
		if m.Version != current {
			continue
		}
		if m.Down == "" {
			return fmt.Errorf("migration %d has no down script", m.Version)
		}
		return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
			if _, err := tx.ExecContext(ctx, m.Down); err != nil {
				return err
			}
			_, err := tx.ExecContext(ctx, `DELETE FROM schema_migrations WHERE version=?`, m.Version)
			return err
		})
	}
	return nil
}

func Status(ctx context.Context, db *bun.DB) ([]int64, error) {
	if err := ensure(ctx, db); err != nil {
		return nil, err
	}
	var versions []int64
	err := db.NewSelect().Table("schema_migrations").Column("version").Order("version ASC").Scan(ctx, &versions)
	return versions, err
}
