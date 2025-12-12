package cmd

import (
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

// rootErrWriter is a writer that returns an error after writing a configured number of bytes
type rootErrWriter struct {
	err          error
	bytesWritten int
	failAfter    int // fail after this many bytes written (-1 to never fail on Write)
}

func (e *rootErrWriter) Write(p []byte) (n int, err error) {
	if e.failAfter >= 0 && e.bytesWritten >= e.failAfter {
		return 0, e.err
	}
	e.bytesWritten += len(p)
	return len(p), nil
}

// writeReservedServiceWarning simulates writing the reserved service warning
func writeReservedServiceWarning(w io.Writer, service string) error {
	if _, err := fmt.Fprintf(w, "Service name %s is reserved for chamber's own use and will be prohibited in a future version. Please switch to a different service name.\n", service); err != nil {
		return fmt.Errorf("failed to write warning: %w", err)
	}
	return nil
}

func TestValidateKey(t *testing.T) {
	validKeyFormat := []string{
		"foo",
		"foo.bar",
		"foo.",
		".foo",
		"foo-bar",
	}

	for _, k := range validKeyFormat {
		t.Run("Key validation should return Nil", func(t *testing.T) {
			result := validateKey(k)
			assert.Nil(t, result)
		})
	}
}

func TestValidateKey_Invalid(t *testing.T) {
	invalidKeyFormat := []string{
		"/foo",
		"foo//bar",
		"foo/bar",
	}

	for _, k := range invalidKeyFormat {
		t.Run("Key validation should return Error", func(t *testing.T) {
			result := validateKey(k)
			assert.Error(t, result)
		})
	}
}

func TestValidateService_Path(t *testing.T) {
	validServicePathFormat := []string{
		"foo",
		"foo.",
		".foo",
		"foo.bar",
		"foo-bar",
		"foo/bar",
		"foo.bar/foo",
		"foo-bar/foo",
		"foo-bar/foo-bar",
		"foo/bar/foo",
		"foo/bar/foo-bar",
		"_chamber", // currently valid, but will be prohibited in a future version
	}

	for _, k := range validServicePathFormat {
		t.Run("Service with PATH validation should return Nil", func(t *testing.T) {
			result := validateService(k)
			assert.Nil(t, result)
		})
	}
}

func TestValidateService_Path_Invalid(t *testing.T) {
	invalidServicePathFormat := []string{
		"foo/",
		"/foo",
		"foo//bar",
	}

	for _, k := range invalidServicePathFormat {
		t.Run("Service with PATH validation should return Error", func(t *testing.T) {
			result := validateService(k)
			assert.Error(t, result)
		})
	}
}

func TestValidateService_PathLabel(t *testing.T) {
	validServicePathFormatWithLabel := []string{
		"foo",
		"foo/bar:-current-",
		"foo.bar/foo:current",
		"foo-bar/foo:current",
		"foo-bar/foo-bar:current",
		"foo/bar/foo:current",
		"foo/bar/foo-bar:current",
		"foo/bar/foo-bar",
		"_chamber", // currently valid, but will be prohibited in a future version
	}

	for _, k := range validServicePathFormatWithLabel {
		t.Run("Service with PATH validation and label should return Nil", func(t *testing.T) {
			result := validateServiceWithLabel(k)
			assert.Nil(t, result)
		})
	}
}

func TestValidateService_PathLabel_Invalid(t *testing.T) {
	invalidServicePathFormatWithLabel := []string{
		"foo:current$",
		"foo.:",
		":foo/bar:current",
		"foo.bar:cur|rent",
	}

	for _, k := range invalidServicePathFormatWithLabel {
		t.Run("Service with PATH validation and label should return Error", func(t *testing.T) {
			result := validateServiceWithLabel(k)
			assert.Error(t, result)
		})
	}
}

func TestReservedServiceWarningWriteErrors(t *testing.T) {
	testErr := errors.New("write error")

	tests := []struct {
		name        string
		service     string
		failAfter   int
		expectError bool
	}{
		{
			name:        "write error on warning",
			service:     "_chamber",
			failAfter:   0,
			expectError: true,
		},
		{
			name:        "write error with different reserved service",
			service:     "_reserved",
			failAfter:   0,
			expectError: true,
		},
		{
			name:        "no error when writer succeeds",
			service:     "_chamber",
			failAfter:   -1,
			expectError: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			w := &rootErrWriter{err: testErr, failAfter: test.failAfter}
			err := writeReservedServiceWarning(w, test.service)

			if test.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), testErr.Error())
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestReservedServiceWarningSuccess(t *testing.T) {
	tests := []struct {
		name    string
		service string
	}{
		{
			name:    "chamber reserved service",
			service: "_chamber",
		},
		{
			name:    "other reserved service",
			service: "_internal",
		},
		{
			name:    "long service name",
			service: "_very_long_reserved_service_name",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			w := &rootErrWriter{failAfter: -1}
			err := writeReservedServiceWarning(w, test.service)
			assert.NoError(t, err)
		})
	}
}

func TestValidateTag(t *testing.T) {
	tests := []struct {
		name        string
		key         string
		value       string
		expectError bool
	}{
		{
			name:        "valid simple tag",
			key:         "Environment",
			value:       "Production",
			expectError: false,
		},
		{
			name:        "valid tag with special chars",
			key:         "aws:cloudformation:stack-name",
			value:       "my-stack/v1.0",
			expectError: false,
		},
		{
			name:        "valid tag with spaces",
			key:         "Team Name",
			value:       "Platform Engineering",
			expectError: false,
		},
		{
			name:        "invalid key with pipe",
			key:         "invalid|key",
			value:       "value",
			expectError: true,
		},
		{
			name:        "invalid value with pipe",
			key:         "key",
			value:       "invalid|value",
			expectError: true,
		},
		{
			name:        "empty key",
			key:         "",
			value:       "value",
			expectError: true,
		},
		{
			name:        "empty value",
			key:         "key",
			value:       "",
			expectError: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateTag(test.key, test.value)
			if test.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
