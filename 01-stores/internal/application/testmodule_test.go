package application_test

import (
	"context"
	"testing"
	"time"

	"github.com/dupakarovsky/eda-storefront/foundation/config"
	"github.com/dupakarovsky/eda-storefront/foundation/logging"
	"github.com/dupakarovsky/eda-storefront/stores"
	"github.com/joho/godotenv"
)

const ApplicationTest string = "application_test.log"

// loadRealEnv will need to load the .ev
func loadRealEnv(t *testing.T) {
	t.Helper()

	//
	if err := godotenv.Load("../../.env"); err != nil {
		t.Fatalf("failed to load 01-stores/.env: %v", err)
	}
}

func newTestModule(logFileName string, t *testing.T) *stores.Module {
	t.Helper()

	// load the .env file before test begins
	loadRealEnv(t)

	// create the config struct for the test
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// create the logger test then close the test is done
	logger, logFile, err := logging.New(logFileName, cfg.Logger.TermLevel, cfg.Logger.FileLevel)
	//logger, logFile, err := logging.New("create_store_retry_test.log", cfg.Logger.TermLevel, cfg.Logger.FileLevel)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	t.Cleanup(func() {
		logFile.Close()
	})

	// define a new context
	ctx := context.Background()

	// initialize the module and cleanup when test is done
	mod, err := stores.New(ctx, cfg, logger)
	if err != nil {
		t.Fatalf("failed to initialize module: %v", err)
	}
	t.Cleanup(func() {
		mod.Close()
	})

	return mod
}

func uniqueID(tag string) string {
	id := tag + "-" + time.Now().Format("20060102150405.000000")
	return id
}
