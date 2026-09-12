package application_test

import (
	"bytes"
	"os/exec"
	"testing"
	"time"

	"github.com/dupakarovsky/eda-storefront/baskets/internal/domain"
)

func dockerComposeContainer(t *testing.T, action, service string) {
	t.Helper()

	cmd := exec.Command("docker", "compose", action, service)
	cmd.Dir = "../../../.."

	out := new(bytes.Buffer)
	cmd.Stdout = out
	cmd.Stderr = out

	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to %s %s container: %v\noutput: %s", action, service, err, out.String())
	}

	t.Logf("%s container %s\noutput: %s", service, action, out.String())
}

func stopNATSContainer(t *testing.T) {
	dockerComposeContainer(t, "stop", "nats")
}

func restartNATSContainerAfter(t *testing.T, delay time.Duration) {
	t.Helper()

	go func() {
		time.Sleep(delay)

		cmd := exec.Command("docker", "compose", "start", "nats")
		cmd.Dir = "../../../.."

		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &out

		if err := cmd.Run(); err != nil {
			t.Errorf("failed to restart nats container: %v\noutput: %s", err, out.String())
			return
		}

		t.Logf("nats container restarted after %s\noutput: %s", delay, out.String())
	}()
}

func TestStartBasket_Execute_Succeeds(t *testing.T) {
	mod := newTestModule(ApplicationTest, t)
	ctx := t.Context()

	id := uniqueID("start-basket-test")

	basket, err := mod.StartBasketUC.Execute(ctx, id)
	if err != nil {
		t.Fatalf("expected Execute to succeed, got error: %v", err)
	}

	if basket.ID != id {
		t.Errorf("expected basket ID %q, got %q", id, basket.ID)
	}
	if basket.Status != domain.BasketOpenStatus {
		t.Errorf("expected status open, got %s", basket.Status)
	}
}

func TestStartBasket_Execute_SucceedsAfterNATSRestart(t *testing.T) {
	mod := newTestModule("start_basket_retry_test.log", t)
	ctx := t.Context()

	stopNATSContainer(t)
	restartNATSContainerAfter(t, 2*time.Second)

	id := uniqueID("start-basket-retry-test")

	basket, err := mod.StartBasketUC.Execute(ctx, id)
	if err != nil {
		t.Fatalf("expected Execute to succeed after NATS restart, got error: %v", err)
	}

	if basket.ID != id {
		t.Errorf("expected basket ID %q, got %q", id, basket.ID)
	}

	t.Logf("basket created successfully after NATS restart: %+v", basket)
}
