package cmd

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	yaml "github.com/goccy/go-yaml"
	"github.com/magiconair/properties"
	analytics "github.com/segmentio/analytics-go/v3"
	"github.com/segmentio/chamber/v3/utils"
	"github.com/spf13/cobra"
)

// exportCmd represents the export command
var (
	exportFormat string
	exportOutput string

	exportCmd = &cobra.Command{
		Use:   "export <service...>",
		Short: "Exports parameters in the specified format",
		Args:  cobra.MinimumNArgs(1),
		RunE:  runExport,
	}
)

func init() {
	exportCmd.Flags().SortFlags = false
	exportCmd.Flags().StringVarP(&exportFormat, "format", "f", "json", "Output format (json, yaml, java-properties, csv, tsv, dotenv, tfvars)")
	exportCmd.Flags().StringVarP(&exportOutput, "output-file", "o", "", "Output file (default is standard output)")

	RootCmd.AddCommand(exportCmd)
}

func runExport(cmd *cobra.Command, args []string) (returnErr error) {
	var err error

	if analyticsEnabled && analyticsClient != nil {
		_ = analyticsClient.Enqueue(analytics.Track{
			UserId: username,
			Event:  "Ran Command",
			Properties: analytics.NewProperties().
				Set("command", "export").
				Set("chamber-version", chamberVersion).
				Set("services", args).
				Set("backend", backend),
		})
	}

	secretStore, err := getSecretStore(cmd.Context())
	if err != nil {
		return err
	}
	params := make(map[string]string)
	for _, service := range args {
		service = utils.NormalizeService(service)
		if err := validateService(service); err != nil {
			return fmt.Errorf("failed to validate service %s: %w", service, err)
		}

		rawSecrets, err := secretStore.ListRaw(cmd.Context(), service)
		if err != nil {
			return fmt.Errorf("failed to list store contents for service %s: %w", service, err)
		}
		for _, rawSecret := range rawSecrets {
			k := key(rawSecret.Key)
			if _, ok := params[k]; ok {
				fmt.Fprintf(os.Stderr, "warning: parameter %s specified more than once (overridden by service %s)\n", k, service)
			}
			params[k] = rawSecret.Value
		}
	}

	file := os.Stdout
	if exportOutput != "" {
		if file, err = os.OpenFile(exportOutput, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644); err != nil {
			return fmt.Errorf("failed to open output file for writing: %w", err)
		}
		defer func() {
			if err := file.Close(); err != nil && returnErr == nil {
				returnErr = fmt.Errorf("failed to close output file: %w", err)
			}
		}()
		defer func() {
			if err := file.Sync(); err != nil && returnErr == nil {
				returnErr = fmt.Errorf("failed to sync output file: %w", err)
			}
		}()
	}
	w := bufio.NewWriter(file)
	defer func() {
		if err := w.Flush(); err != nil && returnErr == nil {
			returnErr = fmt.Errorf("failed to flush output: %w", err)
		}
	}()

	switch strings.ToLower(exportFormat) {
	case "json":
		err = exportAsJson(params, w)
	case "yaml":
		err = exportAsYaml(params, w)
	case "java-properties", "properties":
		err = exportAsJavaProperties(params, w)
	case "csv":
		err = exportAsCsv(params, w)
	case "tsv":
		err = exportAsTsv(params, w)
	case "dotenv":
		err = exportAsEnvFile(params, w)
	case "tfvars":
		err = exportAsTFvars(params, w)
	default:
		err = fmt.Errorf("unsupported export format: %s", exportFormat)
	}

	if err != nil {
		return fmt.Errorf("unable to export parameters: %w", err)
	}

	return nil
}

// this is fundamentally broken, in that there is no actual .env file
// spec. some parsers support values spanned over multiple lines
// as long as they're quoted, others only support character literals
// inside of quotes. we should probably offer the option to control
// which spec we adhere to, or use a marshaler that provides a
// spec instead of hoping for the best.
func exportAsEnvFile(params map[string]string, w io.Writer) error {
	// use top-level escapeSpecials variable to ensure that
	// the dotenv format prints escaped values every time
	escapeSpecials = true
	out, err := buildEnvOutput(params)
	if err != nil {
		return err
	}

	for i := range out {
		_, err := fmt.Fprintln(w, out[i])
		if err != nil {
			return err
		}
	}

	return nil
}

func exportAsTFvars(params map[string]string, w io.Writer) error {
	// Terraform Variables is like dotenv, but removes the TF_VAR and keeps lowercase
	for _, k := range sortedKeys(params) {
		key := sanitizeKey(strings.TrimPrefix(k, "tf_var_"))

		_, err := fmt.Fprintf(w, "%s = \"%s\"\n", key, doubleQuoteEscape(params[k]))
		if err != nil {
			return fmt.Errorf("failed to write variable with key %s: %v", k, err)
		}
	}
	return nil
}

func exportAsJson(params map[string]string, w io.Writer) error {
	// JSON like:
	// {"param1":"value1","param2":"value2"}
	// NOTE: json encoder does sorting by key
	return json.NewEncoder(w).Encode(params)
}

func exportAsYaml(params map[string]string, w io.Writer) error {
	enc := yaml.NewEncoder(w)
	if err := enc.Encode(params); err != nil {
		return err
	}
	return enc.Close()
}

func exportAsJavaProperties(params map[string]string, w io.Writer) error {
	// Java Properties like:
	// param1 = value1
	// param2 = value2
	// ...

	// Load params
	p := properties.NewProperties()
	p.DisableExpansion = true
	for _, k := range sortedKeys(params) {
		_, _, err := p.Set(k, params[k])
		if err != nil {
			return fmt.Errorf("failed to set property %s: %v", k, err)
		}
	}

	// Java expects properties in ISO-8859-1 by default
	_, err := p.Write(w, properties.ISO_8859_1)
	return err
}

func exportAsCsv(params map[string]string, w io.Writer) error {
	// CSV (Comma Separated Values) like:
	// param1,value1
	// param2,value2
	csvWriter := csv.NewWriter(w)
	for _, k := range sortedKeys(params) {
		if err := csvWriter.Write([]string{k, params[k]}); err != nil {
			return fmt.Errorf("failed to write param %q to CSV file: %w", k, err)
		}
	}
	csvWriter.Flush()
	if err := csvWriter.Error(); err != nil {
		return fmt.Errorf("failed to flush CSV output: %w", err)
	}
	return nil
}

func exportAsTsv(params map[string]string, w io.Writer) error {
	// TSV (Tab Separated Values) like:
	tsvWriter := csv.NewWriter(w)
	tsvWriter.Comma = '\t'
	for _, k := range sortedKeys(params) {
		if err := tsvWriter.Write([]string{k, params[k]}); err != nil {
			return fmt.Errorf("failed to write param %q to TSV file: %w", k, err)
		}
	}
	tsvWriter.Flush()
	if err := tsvWriter.Error(); err != nil {
		return fmt.Errorf("failed to flush TSV output: %w", err)
	}
	return nil
}
