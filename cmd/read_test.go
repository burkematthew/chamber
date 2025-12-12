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

// readErrWriter is a writer that returns an error after writing a configured number of bytes
type readErrWriter struct {
	err          error
	bytesWritten int
	failAfter    int // fail after this many bytes written (-1 to never fail on Write)
}

func (e *readErrWriter) Write(p []byte) (n int, err error) {
	if e.failAfter >= 0 && e.bytesWritten >= e.failAfter {
		return 0, e.err
	}
	e.bytesWritten += len(p)
	return len(p), nil
}

// writeReadQuietOutput simulates the quiet mode output from the read function
func writeReadQuietOutput(w io.Writer, value string) error {
	if _, err := fmt.Fprintf(w, "%s\n", value); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}
	return nil
}

// writeReadOutput simulates the normal output writing logic from the read function
func writeReadOutput(w io.Writer, key string, secret store.Secret) (returnErr error) {
	tw := tabwriter.NewWriter(w, 0, 8, 2, '\t', 0)
	defer func() {
		if err := tw.Flush(); err != nil && returnErr == nil {
			returnErr = fmt.Errorf("failed to flush output: %w", err)
		}
	}()

	if _, err := fmt.Fprintln(tw, "Key\tValue\tVersion\tLastModified\tUser"); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}
	if _, err := fmt.Fprintf(tw, "%s\t%s\t%d\t%s\t%s\n",
		key,
		*secret.Value,
		secret.Meta.Version,
		secret.Meta.Created.Local().Format(ShortTimeFormat),
		secret.Meta.CreatedBy); err != nil {
		return fmt.Errorf("failed to write secret: %w", err)
	}
	return nil
}

func TestReadQuietWriteErrors(t *testing.T) {
	testErr := errors.New("write error")

	tests := []struct {
		name        string
		value       string
		failAfter   int
		expectError bool
	}{
		{
			name:        "write error in quiet mode",
			value:       "my-secret-value",
			failAfter:   0,
			expectError: true,
		},
		{
			name:        "write error with long value",
			value:       "this-is-a-very-long-secret-value-that-spans-many-characters",
			failAfter:   0,
			expectError: true,
		},
		{
			name:        "no error when writer succeeds",
			value:       "my-secret-value",
			failAfter:   -1,
			expectError: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			w := &readErrWriter{err: testErr, failAfter: test.failAfter}
			err := writeReadQuietOutput(w, test.value)

			if test.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), testErr.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestReadWriteErrors(t *testing.T) {
	testErr := errors.New("write error")
	testTime := time.Now()
	testValue := "secret-value"

	tests := []struct {
		name        string
		key         string
		secret      store.Secret
		failAfter   int
		expectError bool
	}{
		{
			name: "write error on output",
			key:  "api_key",
			secret: store.Secret{
				Value: &testValue,
				Meta: store.SecretMetadata{
					Created:   testTime,
					CreatedBy: "user1",
					Version:   1,
					Key:       "/service/api_key",
				},
			},
			failAfter:   0,
			expectError: true,
		},
		{
			name: "no error when writer succeeds",
			key:  "api_key",
			secret: store.Secret{
				Value: &testValue,
				Meta: store.SecretMetadata{
					Created:   testTime,
					CreatedBy: "user1",
					Version:   1,
					Key:       "/service/api_key",
				},
			},
			failAfter:   -1,
			expectError: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			w := &readErrWriter{err: testErr, failAfter: test.failAfter}
			err := writeReadOutput(w, test.key, test.secret)

			if test.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), testErr.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestReadFlushError(t *testing.T) {
	testTime := time.Now()
	testValue := "secret-value"

	tests := []struct {
		name      string
		key       string
		secret    store.Secret
		failAfter int
	}{
		{
			name: "flush error with short key",
			key:  "key",
			secret: store.Secret{
				Value: &testValue,
				Meta: store.SecretMetadata{
					Created:   testTime,
					CreatedBy: "user1",
					Version:   1,
					Key:       "/service/key",
				},
			},
			failAfter: 30,
		},
		{
			name: "flush error with long key",
			key:  "very_long_api_key_name",
			secret: store.Secret{
				Value: &testValue,
				Meta: store.SecretMetadata{
					Created:   testTime,
					CreatedBy: "admin@example.com",
					Version:   42,
					Key:       "/service/very_long_api_key_name",
				},
			},
			failAfter: 50,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			testErr := errors.New("io error")
			w := &readErrWriter{err: testErr, failAfter: test.failAfter}
			err := writeReadOutput(w, test.key, test.secret)

			assert.Error(t, err)
			assert.Contains(t, err.Error(), testErr.Error())
		})
	}
}

func TestReadOutputSuccess(t *testing.T) {
	testTime := time.Now()
	testValue1 := "simple-value"
	testValue2 := "value with spaces and special chars: !@#$%"
	testValue3 := ""

	tests := []struct {
		name   string
		key    string
		secret store.Secret
	}{
		{
			name: "simple secret",
			key:  "api_key",
			secret: store.Secret{
				Value: &testValue1,
				Meta: store.SecretMetadata{
					Created:   testTime,
					CreatedBy: "admin",
					Version:   1,
					Key:       "/service/api_key",
				},
			},
		},
		{
			name: "secret with special characters",
			key:  "password",
			secret: store.Secret{
				Value: &testValue2,
				Meta: store.SecretMetadata{
					Created:   testTime,
					CreatedBy: "user@example.com",
					Version:   5,
					Key:       "/service/password",
				},
			},
		},
		{
			name: "empty secret value",
			key:  "empty_key",
			secret: store.Secret{
				Value: &testValue3,
				Meta: store.SecretMetadata{
					Created:   testTime,
					CreatedBy: "system",
					Version:   1,
					Key:       "/service/empty_key",
				},
			},
		},
		{
			name: "high version number",
			key:  "frequently_updated",
			secret: store.Secret{
				Value: &testValue1,
				Meta: store.SecretMetadata{
					Created:   testTime,
					CreatedBy: "ci-system",
					Version:   9999,
					Key:       "/service/frequently_updated",
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			w := &readErrWriter{failAfter: -1}
			err := writeReadOutput(w, test.key, test.secret)
			assert.NoError(t, err)
		})
	}
}

func TestReadQuietOutputSuccess(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{
			name:  "simple value",
			value: "my-secret",
		},
		{
			name:  "empty value",
			value: "",
		},
		{
			name:  "value with newlines",
			value: "line1\nline2\nline3",
		},
		{
			name:  "value with special characters",
			value: "p@ssw0rd!#$%^&*()",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			w := &readErrWriter{failAfter: -1}
			err := writeReadQuietOutput(w, test.value)
			assert.NoError(t, err)
		})
	}
}
