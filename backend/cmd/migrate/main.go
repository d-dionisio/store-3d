package main

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
	if len(os.Args) < 2 {
		fail("usage: go run ./cmd/migrate <up|down|version> [steps]")
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		fail("DATABASE_URL is required")
	}
	migrationsPath := os.Getenv("MIGRATIONS_PATH")
	if migrationsPath == "" {
		migrationsPath = "file://migrations"
	}

	runner, err := migrate.New(migrationsPath, databaseURL)
	if err != nil {
		failWithError("initialize migration runner", err)
	}
	defer func() {
		sourceErr, databaseErr := runner.Close()
		if sourceErr != nil || databaseErr != nil {
			slog.Warn("close migration runner", "source_error", sourceErr, "database_error", databaseErr)
		}
	}()

	switch os.Args[1] {
	case "up":
		err = runner.Up()
	case "down":
		steps, parseErr := downSteps(os.Args[2:])
		if parseErr != nil {
			failWithError("invalid down steps", parseErr)
		}
		err = runner.Steps(-steps)
	case "version":
		version, dirty, versionErr := runner.Version()
		if errors.Is(versionErr, migrate.ErrNilVersion) {
			fmt.Println("version: none")
			return
		}
		if versionErr != nil {
			failWithError("read migration version", versionErr)
		}
		fmt.Printf("version: %d, dirty: %t\n", version, dirty)
		return
	default:
		fail("unknown operation; use up, down, or version")
	}

	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		failWithError("run migration", err)
	}
	if errors.Is(err, migrate.ErrNoChange) {
		fmt.Println("no migration changes")
		return
	}
	fmt.Printf("migration %s completed\n", os.Args[1])
}

func downSteps(arguments []string) (int, error) {
	if len(arguments) == 0 {
		return 1, nil
	}
	steps, err := strconv.Atoi(arguments[0])
	if err != nil || steps <= 0 {
		return 0, errors.New("steps must be a positive integer")
	}
	return steps, nil
}

func fail(message string) {
	slog.Error(message)
	os.Exit(1)
}

func failWithError(message string, err error) {
	slog.Error(message, "error", err)
	os.Exit(1)
}
