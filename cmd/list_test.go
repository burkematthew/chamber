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

// listErrWriter is a writer that returns an error after writing a configured number of bytes
type listErrWriter struct {
	err          error
	bytesWritten int
	failAfter    int // fail after this many bytes written (-1 to never fail on Write)
}

func (e *listErrWriter) Write(p []byte) (n int, err error) {
	if e.failAfter >= 0 && e.bytesWritten >= e.failAfter {
		return 0, e.err
	}
	e.bytesWritten += len(p)
	return len(p), nil
}

// writeListOutput simulates the output writing logic from the list function
// This allows us to test the error handling without needing to mock the secret store
func writeListOutput(w io.Writer, secrets []store.Secret, showValues bool) (returnErr error) {
	tw := tabwriter.NewWriter(w, 0, 8, 2, '\t', 0)
	defer func() {
		if err := tw.Flush(); err != nil && returnErr == nil {
			returnErr = fmt.Errorf("failed to flush output: %w", err)
		}
	}()

	if _, err := fmt.Fprint(tw, "Key\tVersion\tLastModified\tUser"); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}
	if showValues {
		if _, err := fmt.Fprint(tw, "\tValue"); err != nil {
			return fmt.Errorf("failed to write header: %w", err)
		}
	}
	if _, err := fmt.Fprintln(tw, ""); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	for _, secret := range secrets {
		if _, err := fmt.Fprintf(tw, "%s\t%d\t%s\t%s",
			key(secret.Meta.Key),
			secret.Meta.Version,
			secret.Meta.Created.Local().Format(ShortTimeFormat),
			secret.Meta.CreatedBy); err != nil {
			return fmt.Errorf("failed to write secret: %w", err)
		}
		if showValues {
			if _, err := fmt.Fprintf(tw, "\t%s", *secret.Value); err != nil {
				return fmt.Errorf("failed to write secret: %w", err)
			}
		}
		if _, err := fmt.Fprintln(tw, ""); err != nil {
			return fmt.Errorf("failed to write secret: %w", err)
		}
	}

	return nil
}

func TestListWriteErrors(t *testing.T) {
	// Test that write errors are properly propagated.
	testErr := errors.New("write error")
	testTime := time.Now()
	testValue := "secret-value"

	tests := []struct {
		name        string
		secrets     []store.Secret
		showValues  bool
		failAfter   int
		expectError bool
	}{
		{
			name:        "write error with no secrets",
			secrets:     []store.Secret{},
			showValues:  false,
			failAfter:   0,
			expectError: true,
		},
		{
			name: "write error with single secret",
			secrets: []store.Secret{
				{
					Value: &testValue,
					Meta: store.SecretMetadata{
						Created:   testTime,
						CreatedBy: "user1",
						Version:   1,
						Key:       "/service/key1",
					},
				},
			},
			showValues:  false,
			failAfter:   0,
			expectError: true,
		},
		{
			name: "write error with secrets and values",
			secrets: []store.Secret{
				{
					Value: &testValue,
					Meta: store.SecretMetadata{
						Created:   testTime,
						CreatedBy: "user1",
						Version:   1,
						Key:       "/service/key1",
					},
				},
			},
			showValues:  true,
			failAfter:   0,
			expectError: true,
		},
		{
			name: "no error when writer succeeds",
			secrets: []store.Secret{
				{
					Value: &testValue,
					Meta: store.SecretMetadata{
						Created:   testTime,
						CreatedBy: "user1",
						Version:   1,
						Key:       "/service/key1",
					},
				},
			},
			showValues:  false,
			failAfter:   -1, // Never fail
			expectError: false,
		},
		{
			name: "no error when writer succeeds with values",
			secrets: []store.Secret{
				{
					Value: &testValue,
					Meta: store.SecretMetadata{
						Created:   testTime,
						CreatedBy: "user1",
						Version:   1,
						Key:       "/service/key1",
					},
				},
			},
			showValues:  true,
			failAfter:   -1, // Never fail
			expectError: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			w := &listErrWriter{err: testErr, failAfter: test.failAfter}
			err := writeListOutput(w, test.secrets, test.showValues)

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

func TestListFlushError(t *testing.T) {
	// Test that errors occurring during flush are properly returned.
	testTime := time.Now()
	testValue := "secret-value"

	tests := []struct {
		name       string
		secrets    []store.Secret
		showValues bool
		failAfter  int
	}{
		{
			name:       "flush error with no secrets",
			secrets:    []store.Secret{},
			showValues: false,
			failAfter:  20,
		},
		{
			name: "flush error with single secret",
			secrets: []store.Secret{
				{
					Value: &testValue,
					Meta: store.SecretMetadata{
						Created:   testTime,
						CreatedBy: "user1",
						Version:   1,
						Key:       "/service/key1",
					},
				},
			},
			showValues: false,
			failAfter:  40,
		},
		{
			name: "flush error with multiple secrets and values",
			secrets: []store.Secret{
				{
					Value: &testValue,
					Meta: store.SecretMetadata{
						Created:   testTime,
						CreatedBy: "user1",
						Version:   1,
						Key:       "/service/key1",
					},
				},
				{
					Value: &testValue,
					Meta: store.SecretMetadata{
						Created:   testTime,
						CreatedBy: "user2",
						Version:   2,
						Key:       "/service/key2",
					},
				},
			},
			showValues: true,
			failAfter:  80,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			testErr := errors.New("io error")
			w := &listErrWriter{err: testErr, failAfter: test.failAfter}
			err := writeListOutput(w, test.secrets, test.showValues)

			assert.Error(t, err)
			// Verify the underlying error is included
			assert.Contains(t, err.Error(), testErr.Error())
		})
	}
}

func TestListOutputSuccess(t *testing.T) {
	testTime := time.Now()
	testValue1 := "value1"
	testValue2 := "value2"

	tests := []struct {
		name       string
		secrets    []store.Secret
		showValues bool
	}{
		{
			name:       "empty secrets list",
			secrets:    []store.Secret{},
			showValues: false,
		},
		{
			name:       "empty secrets list with values flag",
			secrets:    []store.Secret{},
			showValues: true,
		},
		{
			name: "single secret without values",
			secrets: []store.Secret{
				{
					Value: &testValue1,
					Meta: store.SecretMetadata{
						Created:   testTime,
						CreatedBy: "admin",
						Version:   1,
						Key:       "/myservice/api_key",
					},
				},
			},
			showValues: false,
		},
		{
			name: "single secret with values",
			secrets: []store.Secret{
				{
					Value: &testValue1,
					Meta: store.SecretMetadata{
						Created:   testTime,
						CreatedBy: "admin",
						Version:   1,
						Key:       "/myservice/api_key",
					},
				},
			},
			showValues: true,
		},
		{
			name: "multiple secrets without values",
			secrets: []store.Secret{
				{
					Value: &testValue1,
					Meta: store.SecretMetadata{
						Created:   testTime,
						CreatedBy: "user1",
						Version:   1,
						Key:       "/service/key1",
					},
				},
				{
					Value: &testValue2,
					Meta: store.SecretMetadata{
						Created:   testTime,
						CreatedBy: "user2",
						Version:   3,
						Key:       "/service/key2",
					},
				},
			},
			showValues: false,
		},
		{
			name: "multiple secrets with values",
			secrets: []store.Secret{
				{
					Value: &testValue1,
					Meta: store.SecretMetadata{
						Created:   testTime,
						CreatedBy: "user1",
						Version:   1,
						Key:       "/service/key1",
					},
				},
				{
					Value: &testValue2,
					Meta: store.SecretMetadata{
						Created:   testTime,
						CreatedBy: "user2",
						Version:   3,
						Key:       "/service/key2",
					},
				},
			},
			showValues: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Use a successful writer
			w := &listErrWriter{failAfter: -1}
			err := writeListOutput(w, test.secrets, test.showValues)
			assert.NoError(t, err)
		})
	}
}

func TestKeyFunction(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple key",
			input:    "/service/key",
			expected: "key",
		},
		{
			name:     "nested path",
			input:    "/team/service/subservice/key",
			expected: "key",
		},
		{
			name:     "single segment",
			input:    "key",
			expected: "key",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := key(test.input)
			assert.Equal(t, test.expected, result)
		})
	}
}
