package application_test

import (
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
)

// runCompose runs the "docker compose" command to either
// start to stop the nats container
func runCompose(action string) (string, error) {
	// define a command to run
	cmd := exec.Command("docker", "compose", action, "nats")
	// execute  from the root directory
	cmd.Dir = "../../.."

	// define a slice of bytest to capture the output of the command
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// stopNATS stops the NATS container and restarts it when the test ends
func stopNATS(t *testing.T) {
	t.Helper()

	// run "docker compose stop nats" command
	if out, err := runCompose("stop"); err != nil {
		t.Fatalf("failed to stop nats container: %v\noutput: %s", err, out)
	}

	// Cleanup runs when the test completes. Here i'tll restart the NATS
	// container
	t.Cleanup(func() {
		if out, err := runCompose("start"); err != nil {
			t.Errorf("failed to restore nats container: %v\noutput: %s", err, out)
		}
	})
}

// startNATS runs the "docker componse start nats"
func startNATS(t *testing.T) {
	t.Helper()

	if out, err := runCompose("start"); err != nil {
		t.Fatalf("failed to start nats container: %v\noutput: %s", err, out)
	}
}

// startNATSAfter invokes the runCompose function to start nats
// after a given period of time
func startNATSAfter(t *testing.T, delay time.Duration) {
	t.Helper()

	var wg sync.WaitGroup

	wg.Add(1)
	// goroutine will sleep for a given period a restar the nats container
	go func() {
		defer wg.Done()

		time.Sleep(delay)

		if out, err := runCompose("start"); err != nil {
			t.Errorf("failed to restart nats container: %v\noutput: %s", err, out)
		}
	}()

	// Here we register wg.Wait ot run right away. and it'll wait for
	// the goroutine to complete before unblocking
	t.Cleanup(wg.Wait)
}

func TestRegisterCustomer_Execute_SucceedsAfterNATSRestart(t *testing.T) {

	// initialize a new Moduel
	mod := newTestModule("register_customer_retry_test.log", t)
	ctx := t.Context()

	// get the number of messages in the stream
	before := streamMessageCount(t, ctx, mod)

	// stop the nats container and restart it after 2 seconds
	stopNATS(t)
	startNATSAfter(t, 2*time.Second)

	// unique ID for the customer
	id := uniqueID("register-retry")

	start := time.Now()
	// register the customer
	customer, err := mod.RegisterCustomerUC.Execute(ctx, id, "Alice Smith", "+5531999990000")
	// capture the time it took for the command to complete
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("expected Execute to succeed after NATS restart, got error: %v", err)
	}

	// verify if the cusotmer was registred
	if customer.ID != id {
		t.Errorf("expected customer ID %q, got %q", id, customer.ID)
	}
	// the time it took for the UseCase to run should the above 1 second
	if elapsed < time.Second {
		t.Errorf("expected Execute to outlast the outage, but it returned after only %s", elapsed)
	}

	// verify the number of messages in the stream increased by 1
	after := streamMessageCount(t, ctx, mod)
	if after != before+1 {
		t.Errorf("expected exactly 1 new message for 1 logical publish, got %d", after-before)
	}

	env := lastEnvelope(t, ctx, mod, "customers.customer.CustomerRegistered")
	if env.AggregateID != id {
		t.Errorf("expected the stored event to belong to %s, got %+v", id, env)
	}
}

func TestRegisterCustomer_Execute_ReturnsSavedCustomerWhenRetriesAreExhausted(t *testing.T) {

	// create a new Module
	mod := newTestModule("register_customer_exhausted_test.log", t)
	ctx := t.Context()

	// get the number of messages in the stream
	before := streamMessageCount(t, ctx, mod)

	// stop the nats container
	stopNATS(t)

	id := uniqueID("register-exhausted")

	// try create a new customer; should return an error due to it's inability to publish
	// to the stream; However, the customer was created
	customer, err := mod.RegisterCustomerUC.Execute(ctx, id, "Alice Smith", "+5531999990000")
	if nil == err {
		t.Fatal("expected Execute to fail when NATS stays down, got nil error")
	}
	// should containen an error
	if !strings.Contains(err.Error(), "publishing event") {
		t.Errorf("expected a publish failure, got: %v", err)
	}
	// We expect the customer to have been created
	if customer == nil {
		t.Fatal("expected the saved customer to be returned alongside the error")
	}
	if customer.ID != id {
		t.Errorf("expected customer ID %q, got %q", id, customer.ID)
	}

	// try locating the customer in the database; should find it
	found, findErr := mod.CustomerRepo.FindByID(ctx, id)
	if findErr != nil {
		t.Fatalf("expected the customer to be persisted despite the publish failure, got error: %v", findErr)
	}
	if found.Name != "Alice Smith" {
		t.Errorf("expected persisted name %q, got %q", "Alice Smith", found.Name)
	}

	// close all connections
	mod.Close()

	// restart the nats container
	startNATS(t)

	// define a new moduel
	verify := newTestModule("register_customer_exhausted_verify_test.log", t)

	// verify the number of mesages in the stream remained the same
	if after := streamMessageCount(t, ctx, verify); after != before {
		t.Errorf("expected the event to be missing from the stream, but %d new messages arrived", after-before)
	}
}

const outageLongerThanOldReconnectBudget = 15 * time.Second

func TestNATSConnection_RecoversAfterOutageLongerThanOldReconnectBudget(t *testing.T) {
	// new Module
	mod := newTestModule("nats_reconnect_test.log", t)
	ctx := t.Context()

	// get teh number of messages currently in the stream
	before := streamMessageCount(t, ctx, mod)

	//stop the nats container and sleep for 15s
	stopNATS(t)
	time.Sleep(outageLongerThanOldReconnectBudget)

	// after the stop the status should be RECONNECTING

	// verify if nats has stoped trying to reconnect
	if status := mod.NC.Status(); status == nats.CLOSED {
		t.Fatalf("connection was closed during the outage (status %s): the client exhausted its reconnect budget", status)
	}

	// restart the container
	startNATS(t)

	// sets 10 second deadline after the container has restarted for
	// nats to connect
	deadline := time.Now().Add(10 * time.Second)

	// verify if nats has connected again after 10 seconds;
	// otherwise, it exit the test
	for mod.NC.Status() != nats.CONNECTED {
		if time.Now().After(deadline) {
			t.Fatalf("connection did not recover within 10s of NATS returning (status %s)", mod.NC.Status())
		}
		time.Sleep(100 * time.Millisecond)
	}

	// now try to create the Customer and publish the event
	id := uniqueID("reconnect")
	if _, err := mod.RegisterCustomerUC.Execute(ctx, id, "Alice Smith", "+5531999990000"); err != nil {
		t.Fatalf("expected Execute to succeed after the connection recovered, got error: %v", err)
	}

	// verify if number of messages in the stream has increased
	if after := streamMessageCount(t, ctx, mod); after != before+1 {
		t.Errorf("expected exactly 1 new message, got %d", after-before)
	}
}
