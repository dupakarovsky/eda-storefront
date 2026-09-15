package application_test

import (
	"context"
	"testing"
	"time"

	"github.com/dupakarovsky/eda-storefront/depot"
	"github.com/dupakarovsky/eda-storefront/foundation/config"
	"github.com/dupakarovsky/eda-storefront/foundation/logging"
	"github.com/joho/godotenv"
)

// generated log for the test
const ApplicationTest string = "application_test.log"

// loads the local .env file for accessing the database during
// the tests
func loadRealEnv(t *testing.T) {
	t.Helper()

	if err := godotenv.Load("../../.env"); err != nil {
		t.Fatalf("failed to load depot/.env: %v", err)
	}
}

// newTestModule will initialize a Module struct and return it so
// each test can run
func newTestModule(logFileName string, t *testing.T) *depot.Module {

	// mark this function as a helper
	t.Helper()

	// load the depot's .env files
	loadRealEnv(t)

	// instantiate a Config struct with the using the .env file
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// instantiate a new logger
	logger, logFile, err := logging.New(logFileName, cfg.Logger.TermLevel, cfg.Logger.FileLevel)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}

	// close the logger fille when the test is done
	t.Cleanup(func() {
		logFile.Close()
	})

	ctx := context.Background()

	// instantiate a depot Module
	mod, err := depot.New(ctx, cfg, logger)
	if err != nil {
		t.Fatalf("failed to initialize module: %v", err)
	}

	// close all connections when the test finishes
	t.Cleanup(func() {
		mod.Close()
	})

	return mod
}

// genertes a uniqueID that is timestamped
func uniqueID(tag string) string {
	id := tag + "-" + time.Now().Format("20060102150405.000000")
	return id
}
