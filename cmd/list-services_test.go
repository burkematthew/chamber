package cmd

import (
	"errors"
	"fmt"
	"io"
	"testing"
	"text/tabwriter"

	"github.com/stretchr/testify/assert"
)

// listServicesErrWriter is a writer that returns an error after writing a configured number of bytes
type listServicesErrWriter struct {
	err          error
	bytesWritten int
	failAfter    int // fail after this many bytes written (-1 to never fail on Write)
}

func (e *listServicesErrWriter) Write(p []byte) (n int, err error) {
	if e.failAfter >= 0 && e.bytesWritten >= e.failAfter {
		return 0, e.err
	}
	e.bytesWritten += len(p)
	return len(p), nil
}

// writeListServicesOutput simulates the output writing logic from the listServices function
// This allows us to test the error handling without needing to mock the secret store
func writeListServicesOutput(w io.Writer, services []string) (returnErr error) {
	tw := tabwriter.NewWriter(w, 0, 8, 2, '\t', 0)
	defer func() {
		if err := tw.Flush(); err != nil && returnErr == nil {
			returnErr = fmt.Errorf("failed to flush output: %w", err)
		}
	}()

	if _, err := fmt.Fprint(tw, "Service"); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}
	if _, err := fmt.Fprintln(tw, ""); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	for _, service := range services {
		if _, err := fmt.Fprintf(tw, "%s", service); err != nil {
			return fmt.Errorf("failed to write service: %w", err)
		}
		if _, err := fmt.Fprintln(tw, ""); err != nil {
			return fmt.Errorf("failed to write service: %w", err)
		}
	}
	return nil
}

func TestListServicesWriteErrors(t *testing.T) {
	// Test that write errors are properly propagated.
	// Note: Errors may occur during fmt.Fprint calls or during tabwriter.Flush(),
	// depending on tabwriter's internal buffering behavior.
	testErr := errors.New("write error")

	tests := []struct {
		name        string
		services    []string
		failAfter   int
		expectError bool
	}{
		{
			name:        "write error with no services",
			services:    []string{},
			failAfter:   0,
			expectError: true,
		},
		{
			name:        "write error with single service",
			services:    []string{"service1"},
			failAfter:   0,
			expectError: true,
		},
		{
			name:        "write error with multiple services",
			services:    []string{"service1", "service2", "service3"},
			failAfter:   0,
			expectError: true,
		},
		{
			name:        "no error when writer succeeds with no services",
			services:    []string{},
			failAfter:   -1, // Never fail
			expectError: false,
		},
		{
			name:        "no error when writer succeeds with services",
			services:    []string{"service1", "service2"},
			failAfter:   -1, // Never fail
			expectError: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			w := &listServicesErrWriter{err: testErr, failAfter: test.failAfter}
			err := writeListServicesOutput(w, test.services)

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

func TestListServicesFlushError(t *testing.T) {
	// Test that errors occurring during flush are properly returned.
	// The failAfter values are set high enough to allow initial writes to succeed.
	tests := []struct {
		name      string
		services  []string
		failAfter int
	}{
		{
			name:      "flush error with no services",
			services:  []string{},
			failAfter: 5,
		},
		{
			name:      "flush error with single service",
			services:  []string{"service1"},
			failAfter: 10,
		},
		{
			name:      "flush error with multiple services",
			services:  []string{"service1", "service2", "service3"},
			failAfter: 20,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			testErr := errors.New("io error")
			w := &listServicesErrWriter{err: testErr, failAfter: test.failAfter}
			err := writeListServicesOutput(w, test.services)

			assert.Error(t, err)
			// Verify the underlying error is included
			assert.Contains(t, err.Error(), testErr.Error())
		})
	}
}

func TestListServicesOutputSuccess(t *testing.T) {
	tests := []struct {
		name     string
		services []string
	}{
		{
			name:     "empty services list",
			services: []string{},
		},
		{
			name:     "single service",
			services: []string{"my-service"},
		},
		{
			name:     "multiple services",
			services: []string{"service-a", "service-b", "service-c"},
		},
		{
			name:     "services with nested paths",
			services: []string{"team/app/prod", "team/app/staging", "shared/config"},
		},
		{
			name:     "many services",
			services: []string{"svc1", "svc2", "svc3", "svc4", "svc5", "svc6", "svc7", "svc8", "svc9", "svc10"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Use a successful writer
			w := &listServicesErrWriter{failAfter: -1}
			err := writeListServicesOutput(w, test.services)
			assert.NoError(t, err)
		})
	}
}
