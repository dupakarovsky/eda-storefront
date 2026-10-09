package application_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/dupakarovsky/eda-storefront/customers"
	"github.com/dupakarovsky/eda-storefront/customers/internal/domain"
	customersevents "github.com/dupakarovsky/eda-storefront/customers/internal/events"
	"github.com/dupakarovsky/eda-storefront/foundation/config"
	"github.com/dupakarovsky/eda-storefront/foundation/logging"
	"github.com/joho/godotenv"
)

const ApplicationTest string = "application_test.log"

func loadRealEnv(t *testing.T) {
	t.Helper()

	// load this services environment variables
	if err := godotenv.Load("../../.env"); err != nil {
		t.Fatalf("failed to load customers/.env: %v", err)
	}
}

// creates a new Module struct to run the test
func newTestModule(logFileName string, t *testing.T) *customers.Module {
	t.Helper()

	// load env variables
	loadRealEnv(t)

	// generate an service configuration struct
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// define a new logger
	logger, logFile, err := logging.New(logFileName, cfg.Logger.TermLevel, cfg.Logger.FileLevel)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	// close the log file when test finishes
	t.Cleanup(func() {
		logFile.Close()
	})

	// instantiate a new Module
	mod, err := customers.New(context.Background(), cfg, logger)
	if err != nil {
		t.Fatalf("failed to initialize module: %v", err)
	}

	// run the Close on the module when test finishes
	t.Cleanup(func() {
		mod.Close()
	})

	return mod
}

// generates a new uniqueID for running the test
func uniqueID(tag string) string {
	return tag + "-" + time.Now().Format("20060102150405.000000")
}

func registerTestCustomer(t *testing.T, mod *customers.Module, tag string) *domain.Customer {
	t.Helper()

	// creates a new cusotmer ans saves it to the database to run the test
	customer, err := mod.RegisterCustomerUC.Execute(t.Context(), uniqueID(tag), "Alice Smith", "+5531999990000")
	if err != nil {
		t.Fatalf("failed to register customer: %v", err)
	}

	return customer
}

func streamMessageCount(t *testing.T, ctx context.Context, mod *customers.Module) uint64 {
	t.Helper()

	// start the jetstream for the "customers"
	stream, err := mod.JS.Stream(ctx, customersevents.StreamName)
	if err != nil {
		t.Fatalf("failed to get stream: %v", err)
	}

	// retrieve the the Info object from the stream
	info, err := stream.Info(ctx)
	if err != nil {
		t.Fatalf("failed to get stream info: %v", err)
	}

	// return the number of messages currently in the stream
	return info.State.Msgs
}

func lastEnvelope(t *testing.T, ctx context.Context, mod *customers.Module, subject string) customersevents.Envelope {
	t.Helper()

	// get the jetstream
	stream, err := mod.JS.Stream(ctx, customersevents.StreamName)
	if err != nil {
		t.Fatalf("failed to get stream: %v", err)
	}

	// retrieve the last message stored in the jestream under the subject
	raw, err := stream.GetLastMsgForSubject(ctx, subject)
	if err != nil {
		t.Fatalf("failed to read last message for %s: %v", subject, err)
	}

	// unmarshal the message to get to the Envelope type
	var env customersevents.Envelope
	if err := json.Unmarshal(raw.Data, &env); err != nil {
		t.Fatalf("failed to decode envelope: %v", err)
	}

	// return the Evenlope
	return env
}
