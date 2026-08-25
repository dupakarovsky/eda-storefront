package application_test

import (
	"bytes"
	"context"
	"os/exec"
	"testing"
	"time"

	"github.com/dupakarovsky/eda-storefront/foundation/config"
	"github.com/dupakarovsky/eda-storefront/foundation/logging"
	"github.com/dupakarovsky/eda-storefront/stores"
	"github.com/joho/godotenv"
)

// loadRealEnv will need to load the .ev
func loadRealEnv(t *testing.T) {
	t.Helper()

	//
	if err := godotenv.Load("../../.env"); err != nil {
		t.Fatalf("failed to load 01-stores/.env: %v", err)
	}
}

// dockerComposeContainer is helper that will stop NATS while the test is running
func dockerComposeContainer(t *testing.T, action, service string) {
	t.Helper()

	// command to stop nats
	cmd := exec.Command("docker", "compose", action, service)

	// move to the root to invoke the command: docker-compose.yaml lives at the root dir
	cmd.Dir = "../../.."

	out := new(bytes.Buffer)
	cmd.Stdout = out
	cmd.Stderr = out

	// run the command and wait for it to complete and write to the buffer
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to %s %s container: %v\noutput: %s", action, service, err, out.String())
	}

	t.Logf("nats container stoped\noutput: %s", out.String())
}

// stopNATSContainer will exclusively stop the NATS container
func stopNATSContainer(t *testing.T) {
	dockerComposeContainer(t, "stop", "nats")
}

// startNATSContainer will exclusively start the NATS container
func startNATSContainer(t *testing.T) {
	dockerComposeContainer(t, "start", "nats")
}

// restartNATSContainerAfter will exclusively restart the NATS container
func restartNATSContainerAfter(t *testing.T, delay time.Duration) {
	t.Helper()

	go func() {
		time.Sleep(delay)

		cmd := exec.Command("docker", "compose", "start", "nats")
		cmd.Dir = "../../.."

		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &out

		if err := cmd.Run(); err != nil {
			// Cannot call t.Fatalf here, as it won't kill the test inside a gouroutine
			t.Errorf("failed to restart nats container: %v\noutput: %s", err, out.String())
			return
		}

		t.Logf("nats container restarted after %s\noutput: %s", delay, out.String())
	}()
}

// TestRestartNATSContainer_Helper will test the helper functions
func TestRestartNATSContainer_Helper(t *testing.T) {

	// stop the running container
	stopNATSContainer(t)

	// try to restart the container.
	restartNATSContainerAfter(t, 2*time.Second)

	// give the gouroutine time to finish
	time.Sleep(4 * time.Second)
}

// TestRestartNATSContainer_Helper will test the helper functions
func TestLoadEnv_Helper(t *testing.T) {
	loadRealEnv(t)
}

// TestCreateStore_Execute_SucceedsAfterNATSRestart will try to create a store while NATS is rebooting
func TestCreateStore_Execute_SucceedsAfterNATSRestart(t *testing.T) {
	// load the .env file before test begins
	loadRealEnv(t)

	// create the config struct
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// create the logger
	logger, logFile, err := logging.New("create_store_retry_test.log", cfg.Logger.TermLevel, cfg.Logger.FileLevel)
	if err != nil {
		t.Fatalf("failed to create logger: %v", err)
	}
	defer logFile.Close()

	// define a new context
	ctx := context.Background()

	// initialize the module
	mod, err := stores.New(ctx, cfg, logger)
	if err != nil {
		t.Fatalf("failed to initialize module: %v", err)
	}
	defer mod.Close()

	// stop the NATS container
	stopNATSContainer(t)

	// restart the container after 2 seconds (inside a goroutine to the code
	// continues to execute)

	restartNATSContainerAfter(t, 2*time.Second)

	// define a unique ID for the Store to be created
	id := "store-retry-test-" + time.Now().Format("20060102150405")

	// try to create the Store with the publishRetry; expected to succeed
	store, err := mod.CreateStoreUC.Execute(ctx, id, "Retry Test Grocer", "1 Retry Ave")
	if err != nil {
		t.Fatalf("expected Execute to succeed after NATS restart, got error: %v", err)
	}

	// verify if the created store id matches our provided id
	if store.ID != id {
		t.Errorf("expected store ID %q, got %q", id, store.ID)
	}

	t.Logf("store created successfully after NATS restart: %+v", store)
}
