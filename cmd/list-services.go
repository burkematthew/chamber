package cmd

import (
	"fmt"
	"os"
	"sort"
	"text/tabwriter"

	"github.com/segmentio/chamber/v3/utils"
	"github.com/spf13/cobra"
)

// listServicesCmd represents the list command
var listServicesCmd = &cobra.Command{
	Use:   "list-services <service>",
	Short: "List services",
	RunE:  listServices,
}

var (
	includeSecretName bool
)

func init() {
	listServicesCmd.Flags().BoolVarP(&includeSecretName, "secrets", "s", false, "Include secret names in the list")
	RootCmd.AddCommand(listServicesCmd)
}

func listServices(cmd *cobra.Command, args []string) (returnErr error) {
	var service string
	if len(args) == 0 {
		service = ""
	} else {
		service = utils.NormalizeService(args[0])

	}
	secretStore, err := getSecretStore(cmd.Context())
	if err != nil {
		return fmt.Errorf("failed to get secret store: %w", err)
	}
	secrets, err := secretStore.ListServices(cmd.Context(), service, includeSecretName)
	if err != nil {
		return fmt.Errorf("failed to list store contents: %w", err)
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
	if _, err := fmt.Fprintln(w, ""); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	sort.Strings(secrets)

	for _, secret := range secrets {
		if _, err := fmt.Fprintf(w, "%s", secret); err != nil {
			return fmt.Errorf("failed to write service: %w", err)
		}
		if _, err := fmt.Fprintln(w, ""); err != nil {
			return fmt.Errorf("failed to write service: %w", err)
		}
	}
	return nil
}
