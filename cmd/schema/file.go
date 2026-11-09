package schema

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/Maksim-Gr/kkon/internal/schemaregistry"
	"github.com/spf13/cobra"
)

func fileCommand(publishing bool) *cobra.Command {
	var file, schemaType, references string
	name, description := "check", "Check compatibility using the subject's effective registry policy"
	if publishing {
		name, description = "publish", "Register a schema without a confirmation prompt"
	}
	cmd := &cobra.Command{
		Use: name + " [subject]", Short: description, Args: cobra.MaximumNArgs(1),
		Example: "  kkon schema " + name + " orders-value -f order.avsc\n  kkon schema " + name + " events-value -f event.proto --type protobuf --references refs.json",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := registryClient()
			if err != nil {
				return err
			}
			subject, err := selectSubject(cmd, client, args, publishing)
			if err != nil {
				return err
			}
			inputFile := file
			if inputFile == "" {
				if !canPrompt(cmd) {
					return errors.New("--file is required for non-interactive usage")
				}
				if err := ask(&survey.Input{Message: "Schema file:", Suggest: suggestFiles}, &inputFile); err != nil {
					return err
				}
			}
			inputType := schemaType
			if canPrompt(cmd) && !cmd.Flags().Changed("type") && file == "" {
				if err := ask(&survey.Select{Message: "Schema type:", Options: []string{"avro", "json", "protobuf"}, Default: "avro"}, &inputType); err != nil {
					return err
				}
			}
			input, err := readSchema(inputFile, inputType, references)
			if err != nil {
				return err
			}
			if publishing {
				return publish(cmd, client, subject, input)
			}
			return check(cmd, client, subject, input)
		},
	}
	cmd.Flags().StringVarP(&file, "file", "f", "", "Schema file (raw schema content)")
	cmd.Flags().StringVar(&schemaType, "type", "avro", "Schema type: avro, json, protobuf")
	cmd.Flags().StringVar(&references, "references", "", "JSON file containing registered schema references (name, subject, version)")
	return cmd
}

func suggestFiles(prefix string) []string {
	files, _ := filepath.Glob(prefix + "*")
	return files
}

func readSchema(file, schemaType, referencesFile string) (schemaregistry.Schema, error) {
	input := schemaregistry.Schema{References: []schemaregistry.Reference{}}
	switch strings.ToLower(schemaType) {
	case "avro":
		input.SchemaType = "AVRO"
	case "json":
		input.SchemaType = "JSON"
	case "protobuf":
		input.SchemaType = "PROTOBUF"
	default:
		return input, errors.New("type must be avro, json, or protobuf")
	}
	data, err := os.ReadFile(file) //nolint:gosec // The user explicitly chooses the schema file.
	if err != nil {
		return input, fmt.Errorf("read schema file: %w", err)
	}
	if strings.TrimSpace(string(data)) == "" {
		return input, errors.New("schema file cannot be empty")
	}
	// Leave format validation and reference resolution to the target registry.
	input.Schema = string(data)
	if referencesFile != "" {
		if err := readReferences(referencesFile, &input.References); err != nil {
			return input, err
		}
	}
	return input, nil
}

func readReferences(file string, references *[]schemaregistry.Reference) error {
	data, err := os.ReadFile(file) //nolint:gosec // The user explicitly chooses the references file.
	if err != nil {
		return fmt.Errorf("read references file: %w", err)
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(references); err != nil {
		return fmt.Errorf("references must be a JSON array of name, subject, and version objects: %w", err)
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return errors.New("references file must contain exactly one JSON array")
	}
	if *references == nil {
		return errors.New("references must be a JSON array, not null")
	}
	names := make(map[string]bool)
	for _, ref := range *references {
		if strings.TrimSpace(ref.Name) == "" || strings.TrimSpace(ref.Subject) == "" || ref.Version < 1 {
			return errors.New("each reference requires a nonempty name, subject, and positive version")
		}
		if names[ref.Name] {
			return fmt.Errorf("duplicate reference name %q", ref.Name)
		}
		names[ref.Name] = true
	}
	return nil
}
