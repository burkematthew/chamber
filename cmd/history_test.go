package cmd

import (
	"errors"
	"fmt"
	"io"
	"testing"
	"text/tabwriter"
	"time"

	"github.com/segmentio/chamber/v3/store"
	"github.com/stretchr/testify/assert"
)

// historyErrWriter is a writer that returns an error after writing a configured number of bytes
type historyErrWriter struct {
	err          error
	bytesWritten int
	failAfter    int // fail after this many bytes written (-1 to never fail on Write)
}

func (e *historyErrWriter) Write(p []byte) (n int, err error) {
	if e.failAfter >= 0 && e.bytesWritten >= e.failAfter {
		return 0, e.err
	}
	e.bytesWritten += len(p)
	return len(p), nil
}

// writeHistoryOutput simulates the output writing logic from the history function
// This allows us to test the error handling without needing to mock the secret store
func writeHistoryOutput(w io.Writer, events []store.ChangeEvent) (returnErr error) {
	tw := tabwriter.NewWriter(w, 0, 8, 2, '\t', 0)
	defer func() {
		if err := tw.Flush(); err != nil && returnErr == nil {
			returnErr = fmt.Errorf("failed to flush output: %w", err)
		}
	}()

	if _, err := fmt.Fprintln(tw, "Event\tVersion\tDate\tUser"); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}
	for _, event := range events {
		if _, err := fmt.Fprintf(tw, "%s\t%d\t%s\t%s\n",
			event.Type,
			event.Version,
			event.Time.Local().Format(ShortTimeFormat),
			event.User,
		); err != nil {
			return fmt.Errorf("failed to write event: %w", err)
		}
	}
	return nil
}

func TestHistoryWriteErrors(t *testing.T) {
	// Test that write errors are properly propagated.
	// Note: Errors may occur during fmt.Fprint calls or during tabwriter.Flush(),
	// depending on tabwriter's internal buffering behavior.
	testErr := errors.New("write error")
	testTime := time.Now()

	tests := []struct {
		name        string
		events      []store.ChangeEvent
		failAfter   int
		expectError bool
	}{
		{
			name:        "write error with no events",
			events:      []store.ChangeEvent{},
			failAfter:   0,
			expectError: true,
		},
		{
			name: "write error with single event",
			events: []store.ChangeEvent{
				{Type: store.Created, Time: testTime, User: "user1", Version: 1},
			},
			failAfter:   0,
			expectError: true,
		},
		{
			name: "write error with multiple events",
			events: []store.ChangeEvent{
				{Type: store.Created, Time: testTime, User: "user1", Version: 1},
				{Type: store.Updated, Time: testTime, User: "user2", Version: 2},
			},
			failAfter:   0,
			expectError: true,
		},
		{
			name: "no error when writer succeeds",
			events: []store.ChangeEvent{
				{Type: store.Created, Time: testTime, User: "user1", Version: 1},
			},
			failAfter:   -1, // Never fail
			expectError: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			w := &historyErrWriter{err: testErr, failAfter: test.failAfter}
			err := writeHistoryOutput(w, test.events)

			if test.expectError {
				assert.Error(t, err)
				// Verify the underlying error is included
				assert.Contains(t, err.Error(), testErr.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestHistoryFlushError(t *testing.T) {
	// Test that errors occurring during flush are properly returned.
	// The failAfter values are set high enough to allow initial writes to succeed.
	testTime := time.Now()

	tests := []struct {
		name      string
		events    []store.ChangeEvent
		failAfter int
	}{
		{
			name:      "flush error with no events",
			events:    []store.ChangeEvent{},
			failAfter: 10,
		},
		{
			name: "flush error with single event",
			events: []store.ChangeEvent{
				{Type: store.Created, Time: testTime, User: "user1", Version: 1},
			},
			failAfter: 20,
		},
		{
			name: "flush error with multiple events",
			events: []store.ChangeEvent{
				{Type: store.Created, Time: testTime, User: "user1", Version: 1},
				{Type: store.Updated, Time: testTime, User: "user2", Version: 2},
				{Type: store.Updated, Time: testTime, User: "user3", Version: 3},
			},
			failAfter: 40,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			testErr := errors.New("io error")
			w := &historyErrWriter{err: testErr, failAfter: test.failAfter}
			err := writeHistoryOutput(w, test.events)

			assert.Error(t, err)
			// Verify the underlying error is included
			assert.Contains(t, err.Error(), testErr.Error())
		})
	}
}

func TestHistoryOutputSuccess(t *testing.T) {
	testTime := time.Now()

	tests := []struct {
		name   string
		events []store.ChangeEvent
	}{
		{
			name:   "empty events",
			events: []store.ChangeEvent{},
		},
		{
			name: "single created event",
			events: []store.ChangeEvent{
				{Type: store.Created, Time: testTime, User: "user1", Version: 1},
			},
		},
		{
			name: "single updated event",
			events: []store.ChangeEvent{
				{Type: store.Updated, Time: testTime, User: "user1", Version: 2},
			},
		},
		{
			name: "multiple events",
			events: []store.ChangeEvent{
				{Type: store.Created, Time: testTime, User: "user1", Version: 1},
				{Type: store.Updated, Time: testTime, User: "user2", Version: 2},
				{Type: store.Updated, Time: testTime, User: "user3", Version: 3},
			},
		},
		{
			name: "events with different users",
			events: []store.ChangeEvent{
				{Type: store.Created, Time: testTime, User: "admin@example.com", Version: 1},
				{Type: store.Updated, Time: testTime, User: "developer@example.com", Version: 2},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Use a successful writer
			w := &historyErrWriter{failAfter: -1}
			err := writeHistoryOutput(w, test.events)
			assert.NoError(t, err)
		})
	}
}
