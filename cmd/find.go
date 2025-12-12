package cmd

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/segmentio/chamber/v3/store"
	"github.com/spf13/cobra"
)

// findCmd represents the find command
var findCmd = &cobra.Command{
	Use:   "find <secret name>",
	Short: "Find the given secret across all services",
	Args:  cobra.ExactArgs(1),
	RunE:  find,
}

var (
	blankService   string
	byValue        bool
	includeSecrets bool
	matches        []store.SecretId
)

func init() {
	findCmd.Flags().BoolVarP(&byValue, "by-value", "v", false, "Find parameters by value")
	RootCmd.AddCommand(findCmd)
}

func find(cmd *cobra.Command, args []string) (returnErr error) {
	findSecret := args[0]

	if byValue {
		includeSecrets = false
	} else {
		includeSecrets = true
	}

	secretStore, err := getSecretStore(cmd.Context())
	if err != nil {
		return fmt.Errorf("failed to get secret store: %w", err)
	}
	services, err := secretStore.ListServices(cmd.Context(), blankService, includeSecrets)
	if err != nil {
		return fmt.Errorf("failed to list store contents: %w", err)
	}

	if byValue {
		for _, service := range services {
			allSecrets, err := secretStore.List(cmd.Context(), service, true)
			if err == nil {
				matches = append(matches, findValueMatch(allSecrets, findSecret)...)
			}
		}
	} else {
		matches = append(matches, findKeyMatch(services, findSecret)...)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 8, 2, '\t', 0)
	defer func() {
		if err := w.Flush(); err != nil && returnErr == nil {
			returnErr = fmt.Errorf("failed to flush output: %w", err)
		}
	}()

	if _, err := fmt.Fprint(w, "Service"); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}
	if byValue {
		if _, err := fmt.Fprint(w, "\tKey"); err != nil {
			return fmt.Errorf("failed to write header: %w", err)
		}
	}
	if _, err := fmt.Fprintln(w, ""); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	for _, match := range matches {
		if _, err := fmt.Fprintf(w, "%s", match.Service); err != nil {
			return fmt.Errorf("failed to write match: %w", err)
		}
		if byValue {
			if _, err := fmt.Fprintf(w, "\t%s", match.Key); err != nil {
				return fmt.Errorf("failed to write match: %w", err)
			}
		}
		if _, err := fmt.Fprintln(w, ""); err != nil {
			return fmt.Errorf("failed to write match: %w", err)
		}
	}

	return nil
}

func findKeyMatch(services []string, searchTerm string) []store.SecretId {
	keyMatches := []store.SecretId{}

	for _, service := range services {
		if searchTerm == key(service) {

			keyMatches = append(keyMatches, store.SecretId{
				Service: path(service),
				Key:     key(service),
			})
		}
	}
	return keyMatches
}

func findValueMatch(secrets []store.Secret, searchTerm string) []store.SecretId {
	valueMatches := []store.SecretId{}

	for _, secret := range secrets {
		if *secret.Value == searchTerm {
			valueMatches = append(valueMatches, store.SecretId{
				Service: path(secret.Meta.Key),
				Key:     key(secret.Meta.Key),
			})
		}
	}
	return valueMatches
}

func path(s string) string {
	sep := "/"

	tokens := strings.Split(s, sep)
	secretPath := strings.Join(tokens[1:len(tokens)-1], "/")
	return secretPath
}
