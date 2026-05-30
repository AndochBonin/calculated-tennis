// Build per-player career match JSON under tennisabstract/testdata/career/.
//
// Run (from repo root):
//
//	go run ./cmd/fetchcareer
//	go run ./cmd/fetchcareer -matches=tennisabstract/testdata/atp_matches_2025.csv -dir=tennisabstract/testdata/career
//
// Or: make fetch-career
//
// Use -merge to skip slugs that already have {slug}.json in -dir.
//
// Optional env: TENNISABSTRACT_CAREER_DIR (default dir when -dir is omitted),
// TENNISABSTRACT_REQUEST_INTERVAL (default 2s between requests),
// TENNISABSTRACT_HTTP_MAX_RETRIES, TENNISABSTRACT_HTTP_BACKOFF (429 retry).
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/AndochBonin/E3/tennis/tennisabstract"
	"github.com/joho/godotenv"
)

func main() {
	os.Exit(exitRun())
}

func exitRun() int {
	_ = godotenv.Load()

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	matchesFlag := flag.String("matches", "tennisabstract/testdata/atp_matches_2025.csv", "ATP matches CSV path")
	dirFlag := flag.String("dir", "", "output directory for {slug}.json (default: TENNISABSTRACT_CAREER_DIR or tennisabstract/testdata/career)")
	mergeFlag := flag.Bool("merge", false, "skip slugs that already have a JSON file in -dir")
	flag.Parse()

	dir := *dirFlag
	if dir == "" {
		dir = tennisabstract.CareerCacheDirFromEnv()
	}

	names, err := readUniqueNames(*matchesFlag)
	if err != nil {
		log.Error("read matches csv", "path", *matchesFlag, "err", err)
		return 1
	}
	log.Info("players in csv", "unique_names", len(names), "dir", dir)

	// Empty career cache dir: always live-fetch; we write explicitly after each success.
	opts := tennisabstract.HTTPClientOptionsFromEnv()
	opts = append(opts, tennisabstract.WithCareerCacheDir(""))
	client := tennisabstract.NewClient(opts...)
	ctx := context.Background()

	var fetchErrs int
	var skipped int
	var merged int
	for i, name := range names {
		slug := tennisabstract.PlayerSlug(name)
		if slug == "" {
			log.Warn("skip empty slug", "name", name)
			skipped++
			continue
		}
		if *mergeFlag {
			if _, ok, err := tennisabstract.ReadCareerMatchesFile(dir, slug); err != nil {
				log.Error("read career cache", "slug", slug, "err", err)
				return 1
			} else if ok {
				merged++
				continue
			}
		}

		log.Info("fetching career matches", "progress", fmt.Sprintf("%d/%d", i+1, len(names)), "name", name, "slug", slug)
		career, err := client.GetCareerMatches(ctx, name)
		if err != nil {
			log.Error("get career matches", "name", name, "slug", slug, "err", err)
			fetchErrs++
			continue
		}
		if err := tennisabstract.WriteCareerMatchesFile(dir, slug, career); err != nil {
			log.Error("write career cache", "slug", slug, "err", err)
			fetchErrs++
			continue
		}
	}

	log.Info("done",
		"dir", dir,
		"fetch_errors", fetchErrs,
		"skipped_names", skipped,
		"merged_existing", merged,
	)
	if fetchErrs > 0 {
		return 1
	}
	return 0
}

func readUniqueNames(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return tennisabstract.UniquePlayerNamesFromMatchesCSV(f)
}
