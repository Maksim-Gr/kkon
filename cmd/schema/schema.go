// Package schema provides commands for Confluent Schema Registry workflows.
package schema

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/Maksim-Gr/kkon/cmd/config"
	"github.com/Maksim-Gr/kkon/internal/schemaregistry"
	"github.com/Maksim-Gr/kkon/internal/util"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
)

// NewCommand builds the Schema Registry command group.
func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use: "schema", Short: "Browse, check, and publish Schema Registry schemas",
		Long:              "Manage subjects and schema versions through the Confluent Schema Registry API, independently of Kafka Connect.",
		PersistentPreRunE: prepareRegistry,
	}
	cmd.AddCommand(listCommand(), versionsCommand(), getCommand(), fileCommand(false), fileCommand(true))
	return cmd
}

func prepareRegistry(cmd *cobra.Command, _ []string) error {
	if f := cmd.Root().PersistentFlags().Lookup("output"); f != nil && f.Value.String() != "text" && f.Value.String() != "json" {
		return errors.New("output must be text or json")
	}
	cfg, err := util.ResolveConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	if cfg.SchemaRegistry.URL != "" {
		return util.ValidateURL(cfg.SchemaRegistry.URL)
	}
	if !canPrompt(cmd) {
		return fmt.Errorf("no Schema Registry URL configured: run `kkon config set --schema-registry` or set %s", util.EnvSchemaRegistryURL)
	}
	// Dry-run must not save a configuration as a side effect of initial setup.
	if util.IsDryRun(cmd) {
		return errors.New("configure Schema Registry before using --dry-run: run `kkon config set --schema-registry`")
	}
	return config.ConfigureSchemaRegistry(false)
}

func registryClient() (*schemaregistry.Client, error) {
	cfg, err := util.ResolveConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	r := cfg.SchemaRegistry
	return schemaregistry.NewClient(r.URL, r.Username, r.Password), nil
}

func canPrompt(cmd *cobra.Command) bool {
	return !util.IsJSONOutput(cmd) && util.IsTerminal(os.Stdin)
}

func ask(prompt survey.Prompt, answer any) error {
	if err := survey.AskOne(prompt, answer, survey.WithValidator(survey.Required)); err != nil {
		if util.IsSurveyInterrupt(err) {
			return util.ErrCanceled
		}
		return fmt.Errorf("prompt failed: %w", err)
	}
	return nil
}

func selectSubject(cmd *cobra.Command, client *schemaregistry.Client, args []string, publishing bool) (string, error) {
	if len(args) > 0 {
		if strings.TrimSpace(args[0]) == "" {
			return "", errors.New("subject cannot be empty")
		}
		return args[0], nil
	}
	if !canPrompt(cmd) {
		return "", errors.New("a subject argument is required for non-interactive usage")
	}
	var subject string
	subjects, err := client.List(cmd.Context())
	if err != nil {
		return "", err
	}
	if publishing {
		var index int
		options := append([]string{}, subjects...)
		options = append(options, "Create a new subject...")
		if len(subjects) > 0 {
			if err := ask(&survey.Select{Message: "Subject:", Options: options}, &index); err != nil {
				return "", err
			}
			if index < len(subjects) {
				return subjects[index], nil
			}
		}
		err := ask(&survey.Input{Message: "New subject name:"}, &subject)
		return subject, err
	}
	if len(subjects) == 0 {
		return "", errors.New("no subjects found in Schema Registry")
	}
	err = ask(&survey.Select{Message: "Subject:", Options: subjects}, &subject)
	return subject, err
}

func printJSON(cmd *cobra.Command, value any) error {
	encoder := json.NewEncoder(cmd.OutOrStdout())
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}
