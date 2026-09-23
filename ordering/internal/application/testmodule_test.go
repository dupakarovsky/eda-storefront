// ordering/internal/application/testmodule_test.go
package application_test

import (
	"context"
	"testing"
	"time"

	"github.com/dupakarovsky/eda-storefront/foundation/config"
	"github.com/dupakarovsky/eda-storefront/foundation/logging"
	"github.com/dupakarovsky/eda-storefront/ordering"
	"github.com/joho/godotenv"
)

const ApplicationTest string = "application_test.log"

func loadRealEnv(t *testing.T) {
	t.Helper()

	if err := godotenv.Load("../../.env"); err != nil {
		t.Fatalf("failed to load ordering/.env: %v", err)
	}
}

func newTestModule(logFileName string, t *testing.T) *ordering.Module {
	t.Helper()

	loadRealEnv(t)

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	logger, logFile, err := logging.New(logFileName, cfg.Logger.TermLevel, cfg.Logger.FileLevel)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}

	t.Cleanup(func() {
		logFile.Close()
	})

	ctx := context.Background()

	mod, err := ordering.New(ctx, cfg, logger)
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
