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

func TestFindFunctions(t *testing.T) {
	tests := []struct {
		name   string
		params string
		output string
	}{
		{name: "service1", params: "/service1/key_one", output: "service1"},
		{name: "service2", params: "/service2/subService/key_two", output: "service2/subService"},
		{name: "service3", params: "/service3/subService/subSubService/key_three", output: "service3/subService/subSubService"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := path(test.params)
			assert.Equal(t, test.output, result)
		})
	}

	keyMatchTests := []struct {
		name       string
		services   []string
		searchTerm string
		output     []store.SecretId
	}{
		{
			name: "findNoMatches",
			services: []string{
				"/service1/launch_darkly_key",
				"/service2/s3_bucket_base",
				"/service3/slack_token",
			},
			searchTerm: "s3_bucket",
			output:     []store.SecretId{},
		},
		{
			name: "findSomeMatches",
			services: []string{
				"/service1/s3_bucket",
				"/service2/s3_bucket_base",
				"/service3/s3_bucket",
			},
			searchTerm: "s3_bucket",
			output: []store.SecretId{
				{Service: "service1", Key: "s3_bucket"},
				{Service: "service3", Key: "s3_bucket"},
			},
		},
		{
			name: "findEverythingMatches",
			services: []string{
				"/service1/s3_bucket",
				"/service2/s3_bucket",
				"/service3/s3_bucket",
			},
			searchTerm: "s3_bucket",
			output: []store.SecretId{
				{Service: "service1", Key: "s3_bucket"},
				{Service: "service2", Key: "s3_bucket"},
				{Service: "service3", Key: "s3_bucket"},
			},
		},
	}

	for _, test := range keyMatchTests {
		t.Run(test.name, func(t *testing.T) {
			result := findKeyMatch(test.services, test.searchTerm)
			fmt.Println(result)
			assert.Equal(t, test.output, result)
		})
	}

	valueDarklyToken := "1@m@Pr3t3ndL@unchD@rkl3yK3y"
	valueSlackToken := "1@m@Pr3t3ndSlackToken"
	valueGoodS3Bucket := "s3://this_bucket"
	valueBadS3Bucket := "s3://not_your_bucket"

	valueMatchTests := []struct {
		name       string
		secrets    []store.Secret
		searchTerm string
		output     []store.SecretId
	}{
		{
			name: "findNoMatches",
			secrets: []store.Secret{
				{
					Value: &valueDarklyToken,
					Meta: store.SecretMetadata{
						Created:   time.Now(),
						CreatedBy: "no one",
						Version:   0,
						Key:       "/service1/launch_darkly_key",
					},
				},
				{
					Value: &valueSlackToken,
					Meta: store.SecretMetadata{
						Created:   time.Now(),
						CreatedBy: "no one",
						Version:   0,
						Key:       "/service1/slack_token",
					},
				},
				{
					Value: &valueBadS3Bucket,
					Meta: store.SecretMetadata{
						Created:   time.Now(),
						CreatedBy: "no one",
						Version:   0,
						Key:       "/service1/s3_bucket",
					},
				},
			},
			searchTerm: "s3://this_bucket",
			output:     []store.SecretId{},
		},
		{
			"findSomeMatches",
			[]store.Secret{
				{
					Value: &valueDarklyToken,
					Meta: store.SecretMetadata{
						Created:   time.Now(),
						CreatedBy: "no one",
						Version:   0,
						Key:       "/service1/launch_darkly_key",
					},
				},
				{
					Value: &valueGoodS3Bucket,
					Meta: store.SecretMetadata{
						Created:   time.Now(),
						CreatedBy: "no one",
						Version:   0,
						Key:       "/service1/s3_bucket_name",
					},
				},
				{
					Value: &valueGoodS3Bucket,
					Meta: store.SecretMetadata{
						Created:   time.Now(),
						CreatedBy: "no one",
						Version:   0,
						Key:       "/service1/s3_bucket",
					},
				},
			},
			"s3://this_bucket",
			[]store.SecretId{
				{
					Service: "service1",
					Key:     "s3_bucket_name",
				},
				{
					Service: "service1",
					Key:     "s3_bucket",
				},
			},
		},
		{
			"findEverythingMatches",
			[]store.Secret{
				{
					Value: &valueGoodS3Bucket,
					Meta: store.SecretMetadata{
						Created:   time.Now(),
						CreatedBy: "no one",
						Version:   0,
						Key:       "/service1/s3_bucket_base",
					},
				},
				{
					Value: &valueGoodS3Bucket,
					Meta: store.SecretMetadata{
						Created:   time.Now(),
						CreatedBy: "no one",
						Version:   0,
						Key:       "/service1/s3_bucket_name",
					},
				},
				{
					Value: &valueGoodS3Bucket,
					Meta: store.SecretMetadata{
						Created:   time.Now(),
						CreatedBy: "no one",
						Version:   0,
						Key:       "/service1/s3_bucket",
					},
				},
			},
			"s3://this_bucket",
			[]store.SecretId{
				{
					Service: "service1",
					Key:     "s3_bucket_base",
				},
				{
					Service: "service1",
					Key:     "s3_bucket_name",
				},
				{
					Service: "service1",
					Key:     "s3_bucket",
				},
			},
		},
	}

	for _, test := range valueMatchTests {
		t.Run(test.name, func(t *testing.T) {
			result := findValueMatch(test.secrets, test.searchTerm)
			assert.Equal(t, test.output, result)
		})
	}

}

// findErrWriter is a writer that returns an error after writing a configured number of bytes
type findErrWriter struct {
	err          error
	bytesWritten int
	failAfter    int // fail after this many bytes written (-1 to never fail on Write)
}

func (e *findErrWriter) Write(p []byte) (n int, err error) {
	if e.failAfter >= 0 && e.bytesWritten >= e.failAfter {
		return 0, e.err
	}
	e.bytesWritten += len(p)
	return len(p), nil
}

// writeFindOutput simulates the output writing logic from the find function
// This allows us to test the error handling without needing to mock the secret store
func writeFindOutput(w io.Writer, matchList []store.SecretId, byVal bool) (returnErr error) {
	tw := tabwriter.NewWriter(w, 0, 8, 2, '\t', 0)
	defer func() {
		if err := tw.Flush(); err != nil && returnErr == nil {
			returnErr = fmt.Errorf("failed to flush output: %w", err)
		}
	}()

	if _, err := fmt.Fprint(tw, "Service"); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}
	if byVal {
		if _, err := fmt.Fprint(tw, "\tKey"); err != nil {
			return fmt.Errorf("failed to write header: %w", err)
		}
	}
	if _, err := fmt.Fprintln(tw, ""); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	for _, match := range matchList {
		if _, err := fmt.Fprintf(tw, "%s", match.Service); err != nil {
			return fmt.Errorf("failed to write match: %w", err)
		}
		if byVal {
			if _, err := fmt.Fprintf(tw, "\t%s", match.Key); err != nil {
				return fmt.Errorf("failed to write match: %w", err)
			}
		}
		if _, err := fmt.Fprintln(tw, ""); err != nil {
			return fmt.Errorf("failed to write match: %w", err)
		}
	}

	return nil
}

func TestFindWriteErrors(t *testing.T) {
	// Test that write errors are properly propagated.
	// Note: Errors may occur during fmt.Fprint calls or during tabwriter.Flush(),
	// depending on tabwriter's internal buffering behavior.
	testErr := errors.New("write error")

	tests := []struct {
		name        string
		matches     []store.SecretId
		byValue     bool
		failAfter   int
		expectError bool
	}{
		{
			name:        "write error with empty matches",
			matches:     []store.SecretId{},
			byValue:     false,
			failAfter:   0,
			expectError: true,
		},
		{
			name:        "write error with byValue flag",
			matches:     []store.SecretId{},
			byValue:     true,
			failAfter:   0,
			expectError: true,
		},
		{
			name: "write error with matches",
			matches: []store.SecretId{
				{Service: "service1", Key: "key1"},
			},
			byValue:     false,
			failAfter:   0,
			expectError: true,
		},
		{
			name: "write error with matches and byValue",
			matches: []store.SecretId{
				{Service: "service1", Key: "key1"},
			},
			byValue:     true,
			failAfter:   0,
			expectError: true,
		},
		{
			name: "no error when writer succeeds",
			matches: []store.SecretId{
				{Service: "service1", Key: "key1"},
			},
			byValue:     false,
			failAfter:   -1, // Never fail
			expectError: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			w := &findErrWriter{err: testErr, failAfter: test.failAfter}
			err := writeFindOutput(w, test.matches, test.byValue)

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

func TestFindFlushError(t *testing.T) {
	// Test that errors occurring during flush are properly returned.
	// The failAfter values are set high enough to allow initial writes to succeed.
	tests := []struct {
		name      string
		matches   []store.SecretId
		byValue   bool
		failAfter int
	}{
		{
			name:      "flush error with empty matches",
			matches:   []store.SecretId{},
			byValue:   false,
			failAfter: 5,
		},
		{
			name: "flush error with single match",
			matches: []store.SecretId{
				{Service: "service1", Key: "key1"},
			},
			byValue:   false,
			failAfter: 15,
		},
		{
			name: "flush error with multiple matches and byValue",
			matches: []store.SecretId{
				{Service: "service1", Key: "key1"},
				{Service: "service2", Key: "key2"},
			},
			byValue:   true,
			failAfter: 25,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			testErr := errors.New("io error")
			w := &findErrWriter{err: testErr, failAfter: test.failAfter}
			err := writeFindOutput(w, test.matches, test.byValue)

			assert.Error(t, err)
			// Verify the underlying error is included
			assert.Contains(t, err.Error(), testErr.Error())
		})
	}
}

func TestFindOutputSuccess(t *testing.T) {
	tests := []struct {
		name    string
		matches []store.SecretId
		byValue bool
	}{
		{
			name:    "empty matches without byValue",
			matches: []store.SecretId{},
			byValue: false,
		},
		{
			name:    "empty matches with byValue",
			matches: []store.SecretId{},
			byValue: true,
		},
		{
			name: "single match without byValue",
			matches: []store.SecretId{
				{Service: "service1", Key: "key1"},
			},
			byValue: false,
		},
		{
			name: "single match with byValue",
			matches: []store.SecretId{
				{Service: "service1", Key: "key1"},
			},
			byValue: true,
		},
		{
			name: "multiple matches without byValue",
			matches: []store.SecretId{
				{Service: "service1", Key: "key1"},
				{Service: "service2", Key: "key2"},
				{Service: "service3/sub", Key: "key3"},
			},
			byValue: false,
		},
		{
			name: "multiple matches with byValue",
			matches: []store.SecretId{
				{Service: "service1", Key: "key1"},
				{Service: "service2", Key: "key2"},
				{Service: "service3/sub", Key: "key3"},
			},
			byValue: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Use a successful writer (bytes.Buffer would work, but we use our errWriter with failAfter=-1)
			w := &findErrWriter{failAfter: -1}
			err := writeFindOutput(w, test.matches, test.byValue)
			assert.NoError(t, err)
		})
	}
}
