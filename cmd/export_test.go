package cmd

import (
	"bufio"
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// errWriter is a writer that returns an error after writing a configured number of bytes
type errWriter struct {
	err          error
	bytesWritten int
	failAfter    int // fail after this many bytes written (-1 to never fail on Write)
}

func (e *errWriter) Write(p []byte) (n int, err error) {
	if e.failAfter >= 0 && e.bytesWritten >= e.failAfter {
		return 0, e.err
	}
	e.bytesWritten += len(p)
	return len(p), nil
}

func TestExportDotenv(t *testing.T) {
	tests := []struct {
		name   string
		params map[string]string
		output []string
	}{
		{
			name:   "simple string, simple test",
			params: map[string]string{"foo": "bar"},
			output: []string{`FOO="bar"`},
		},
		{
			name:   "literal dollar signs should be properly escaped",
			params: map[string]string{"foo": "bar", "baz": `$qux`},
			output: []string{`FOO="bar"`, `BAZ="\$qux"`},
		},
		{
			name:   "double quotes should be fully escaped",
			params: map[string]string{"foo": "bar", "baz": `"qux"`},
			output: []string{`FOO="bar"`, `BAZ="\"qux\""`},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			err := exportAsEnvFile(test.params, buf)

			assert.Nil(t, err)
			assert.ElementsMatch(t, test.output, strings.Split(strings.TrimSpace(buf.String()), "\n"))
		})
	}
}

func TestExportWriteErrors(t *testing.T) {
	testErr := errors.New("write error")
	params := map[string]string{"foo": "bar", "baz": "qux"}

	tests := []struct {
		name       string
		exportFunc func(map[string]string, *errWriter) error
	}{
		{
			name: "exportAsJson returns write errors",
			exportFunc: func(p map[string]string, w *errWriter) error {
				return exportAsJson(p, w)
			},
		},
		// Note: exportAsYaml uses goccy/go-yaml which buffers internally
		// and may not immediately propagate write errors
		{
			name: "exportAsJavaProperties returns write errors",
			exportFunc: func(p map[string]string, w *errWriter) error {
				return exportAsJavaProperties(p, w)
			},
		},
		{
			name: "exportAsCsv returns write errors",
			exportFunc: func(p map[string]string, w *errWriter) error {
				return exportAsCsv(p, w)
			},
		},
		{
			name: "exportAsTsv returns write errors",
			exportFunc: func(p map[string]string, w *errWriter) error {
				return exportAsTsv(p, w)
			},
		},
		{
			name: "exportAsEnvFile returns write errors",
			exportFunc: func(p map[string]string, w *errWriter) error {
				return exportAsEnvFile(p, w)
			},
		},
		{
			name: "exportAsTFvars returns write errors",
			exportFunc: func(p map[string]string, w *errWriter) error {
				return exportAsTFvars(p, w)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			w := &errWriter{err: testErr, failAfter: 0}
			err := test.exportFunc(params, w)
			assert.Error(t, err)
		})
	}
}

func TestExportFunctionsSucceedWithValidWriter(t *testing.T) {
	params := map[string]string{"foo": "bar", "baz": "qux"}

	tests := []struct {
		name       string
		exportFunc func(map[string]string, *bytes.Buffer) error
	}{
		{
			name: "exportAsJson succeeds",
			exportFunc: func(p map[string]string, w *bytes.Buffer) error {
				return exportAsJson(p, w)
			},
		},
		{
			name: "exportAsYaml succeeds",
			exportFunc: func(p map[string]string, w *bytes.Buffer) error {
				return exportAsYaml(p, w)
			},
		},
		{
			name: "exportAsJavaProperties succeeds",
			exportFunc: func(p map[string]string, w *bytes.Buffer) error {
				return exportAsJavaProperties(p, w)
			},
		},
		{
			name: "exportAsCsv succeeds",
			exportFunc: func(p map[string]string, w *bytes.Buffer) error {
				return exportAsCsv(p, w)
			},
		},
		{
			name: "exportAsTsv succeeds",
			exportFunc: func(p map[string]string, w *bytes.Buffer) error {
				return exportAsTsv(p, w)
			},
		},
		{
			name: "exportAsEnvFile succeeds",
			exportFunc: func(p map[string]string, w *bytes.Buffer) error {
				return exportAsEnvFile(p, w)
			},
		},
		{
			name: "exportAsTFvars succeeds",
			exportFunc: func(p map[string]string, w *bytes.Buffer) error {
				return exportAsTFvars(p, w)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			err := test.exportFunc(params, buf)
			assert.NoError(t, err)
			assert.NotEmpty(t, buf.String())
		})
	}
}

func TestBufioWriterFlushError(t *testing.T) {
	// This test verifies that errors during bufio.Writer.Flush() are properly detected.
	// When the underlying writer fails during Flush, the error should be captured.
	tests := []struct {
		name         string
		failAfter    int
		expectFlush  error
		expectEncode error
	}{
		{
			name:         "flush fails when underlying writer fails after buffering",
			failAfter:    100, // Accept initial writes, fail later
			expectEncode: nil,
			expectFlush:  errors.New("flush error"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			params := map[string]string{"foo": "bar"}
			ew := &errWriter{err: test.expectFlush, failAfter: test.failAfter}
			w := bufio.NewWriter(ew)

			// Write data that will be buffered
			err := exportAsJson(params, w)
			if test.expectEncode != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			// Force the errWriter to fail on next write
			ew.failAfter = 0

			// Now Flush should fail when it tries to write to the underlying writer
			err = w.Flush()
			if test.expectFlush != nil {
				assert.Error(t, err)
				assert.Equal(t, test.expectFlush, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExportWriteErrorMessages(t *testing.T) {
	// Test that specific export functions return appropriate error messages
	testErr := errors.New("disk full")
	params := map[string]string{"testkey": "testvalue"}

	tests := []struct {
		name           string
		exportFunc     func(map[string]string, *errWriter) error
		expectedSubstr string
	}{
		{
			name: "exportAsTFvars wraps error with key info",
			exportFunc: func(p map[string]string, w *errWriter) error {
				return exportAsTFvars(p, w)
			},
			expectedSubstr: "failed to write variable",
		},
		{
			name: "exportAsCsv wraps error with flush info",
			exportFunc: func(p map[string]string, w *errWriter) error {
				return exportAsCsv(p, w)
			},
			expectedSubstr: "failed to flush CSV",
		},
		{
			name: "exportAsTsv wraps error with flush info",
			exportFunc: func(p map[string]string, w *errWriter) error {
				return exportAsTsv(p, w)
			},
			expectedSubstr: "failed to flush TSV",
		},
		{
			name: "exportAsJavaProperties returns write error",
			exportFunc: func(p map[string]string, w *errWriter) error {
				return exportAsJavaProperties(p, w)
			},
			expectedSubstr: "disk full", // Java properties writer returns the underlying error
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			w := &errWriter{err: testErr, failAfter: 0}
			err := test.exportFunc(params, w)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), test.expectedSubstr)
		})
	}
}
