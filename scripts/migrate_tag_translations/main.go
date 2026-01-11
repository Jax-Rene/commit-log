package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/commitlog/internal/db"
	"github.com/commitlog/internal/service"
)

func main() {
	var dbPath string
	var langs string
	flag.StringVar(&dbPath, "db", "commitlog.db", "sqlite db path")
	flag.StringVar(&langs, "langs", "zh,en", "comma-separated languages to backfill (e.g. zh,en)")
	flag.Parse()

	if err := db.Init(dbPath); err != nil {
		fmt.Fprintf(os.Stderr, "init db: %v\n", err)
		os.Exit(1)
	}

	tagSvc := service.NewTagService(db.DB)
	languages := splitCSV(langs)
	created, err := tagSvc.BackfillTagTranslations(languages)
	if err != nil {
		fmt.Fprintf(os.Stderr, "backfill tag translations: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("done: created %d tag translations\n", created)
}

func splitCSV(value string) []string {
	raw := strings.Split(value, ",")
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	return out
}
