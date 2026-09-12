package application_test

import (
	"context"
	"testing"
	"time"

	"github.com/dupakarovsky/eda-storefront/baskets"
	"github.com/dupakarovsky/eda-storefront/foundation/config"
	"github.com/dupakarovsky/eda-storefront/foundation/logging"
	"github.com/joho/godotenv"
)

const ApplicationTest string = "application_test.log"

// loadRealEnv will load the "baskets" local .env file needed
// to construct the module
func loadRealEnv(t *testing.T) {
	// mark this function as a test helper
	t.Helper()

	// load local .env files
	if err := godotenv.Load("../../.env"); err != nil {
		t.Fatalf("failed to load baskets/.env: %v", err)
	}
}

// newTestModule will return a basket.Module needed for running the tests
func newTestModule(logFileName string, t *testing.T) *baskets.Module {
	t.Helper()

	// load .env file
	loadRealEnv(t)

	// creaate the Config struct
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// initialize the logger
	logger, logFile, err := logging.New(logFileName, cfg.Logger.TermLevel, cfg.Logger.FileLevel)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}

	// close the log file when the test is done
	t.Cleanup(func() {
		logFile.Close()
	})

	ctx := context.Background()

	// initialize a new module
	mod, err := baskets.New(ctx, cfg, logger)
	if err != nil {
		t.Fatalf("failed to initialize module: %v", err)
	}

	// close all connections when test concludes
	t.Cleanup(func() {
		mod.Close()
	})

	return mod
}

func uniqueID(tag string) string {
	id := tag + "-" + time.Now().Format("20060102150405.000000")
	return id
}
