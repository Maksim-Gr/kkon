package connector

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/Maksim-Gr/kkon/internal/connector"
	template "github.com/Maksim-Gr/kkon/internal/connector/kafka/templates"
	"github.com/Maksim-Gr/kkon/internal/util"

	"github.com/AlecAivazis/survey/v2"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

// Available connectors.
var connectors = []string{
	"RabbitMQ Connector",
	"Debezium PostgreSQL CDC",
	"JDBC Source Connector",
	"JDBC Sink Connector",
	"S3 Sink Connector",
}

const customPluginOption = "Custom (choose from installed plugins)"

var (
	connectorJSONPath string
	createPluginClass string
)

// CreateCmd represents the create command.
var CreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a connector from a template, plugin, or JSON file",
	Long: `Create a connector on Kafka Connect.

Without flags, an interactive wizard offers predefined templates or any plugin
installed on the cluster. Pass --plugin to configure a specific plugin class,
or --file to submit a JSON config non-interactively.`,
	Example: `  # Interactive wizard with predefined templates
  kkon connector create

  # Configure a specific installed plugin (see 'kkon connector plugins')
  kkon connector create --plugin io.debezium.connector.postgresql.PostgresConnector

  # Create from a JSON config file
  kkon connector create --file ./my-connector.json

  # Preview without creating anything
  kkon connector create --file ./my-connector.json --dry-run`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		if connectorJSONPath != "" {
			return submitConnectorFromFile(cmd, connectorJSONPath)
		}

		if util.IsJSONOutput(cmd) {
			return errors.New("--output json requires --file (the create wizard is interactive)")
		}

		if createPluginClass != "" {
			return configureFromPlugin(cmd, createPluginClass)
		}

		var selected string
		color.Cyan("\n Available Kafka Connectors \n")
		prompt := &survey.Select{
			Message: "Pick a connector to work with:",
			Options: append(append([]string{}, connectors...), customPluginOption),
		}
		if err := survey.AskOne(prompt, &selected); err != nil {
			return util.ErrCanceled
		}
		color.Green("\n You selected: %s\n", selected)
		switch selected {
		case "RabbitMQ Connector":
			return configureConnector(cmd, "RabbitMQ Connector", template.GetRabbitMQConnectorTemplate(), template.RabbitMQRequiredFields(), "rabbitmq.password")
		case "Debezium PostgreSQL CDC":
			return configureConnector(cmd, "Debezium PostgreSQL CDC", template.GetDebeziumPostgresConnectorTemplate(), template.DebeziumPostgresRequiredFields(), "database.password")
		case "JDBC Source Connector":
			return configureConnector(cmd, "JDBC Source Connector", template.GetJDBCSourceConnectorTemplate(), template.JDBCSourceRequiredFields(), "connection.password")
		case "JDBC Sink Connector":
			return configureConnector(cmd, "JDBC Sink Connector", template.GetJDBCSinkConnectorTemplate(), template.JDBCSinkRequiredFields(), "connection.password")
		case "S3 Sink Connector":
			return configureConnector(cmd, "S3 Sink Connector", template.GetS3SinkConnectorTemplate(), template.S3SinkRequiredFields(), "")
		case customPluginOption:
			return configureFromPlugin(cmd, "")
		}
		return nil
	},
}

func init() {
	CreateCmd.Flags().StringVarP(&connectorJSONPath, "file", "f", "", "Path to connector JSON config file")
	CreateCmd.Flags().StringVar(&createPluginClass, "plugin", "", "Connector plugin class to configure (see 'kkon connector plugins')")
}

func submitConnectorFromFile(cmd *cobra.Command, path string) error {
	ctx := cmd.Context()
	jsonMode := util.IsJSONOutput(cmd)

	b, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", path, err)
	}

	var js json.RawMessage
	if err := json.Unmarshal(b, &js); err != nil {
		return fmt.Errorf("invalid JSON in %s: %w", path, err)
	}

	var meta struct {
		Name string `json:"name"`
	}
	_ = json.Unmarshal(b, &meta)

	client, err := util.NewKafkaConnectClient()
	if err != nil {
		return err
	}

	configMap, wrapped := configMapFromFile(b)
	payload := b
	// POST /connectors only accepts {"name","config"}; wrap a flat config map.
	if configMap != nil && !wrapped {
		if configMap["name"] == "" {
			return fmt.Errorf("connector config in %s has no \"name\"", path)
		}
		meta.Name = configMap["name"]
		if payload, err = json.Marshal(map[string]any{"name": meta.Name, "config": configMap}); err != nil {
			return fmt.Errorf("failed to encode config: %w", err)
		}
	}

	if configMap != nil {
		if jsonMode {
			if err := validateConfigStrict(ctx, client, configMap); err != nil {
				return err
			}
		} else if !validateConfigOrConfirm(ctx, client, configMap) {
			color.Yellow("Submission cancelled.\n")
			return util.ErrNothingToDo
		}
	}

	if util.IsDryRun(cmd) {
		if jsonMode {
			printDryRunPreview(jsonMode, "create", []string{meta.Name}, "")
			return nil
		}
		color.Yellow("[dry-run] Would submit connector from file %s\n", path)
		return nil
	}

	if !jsonMode {
		color.Green("\n Submitting connector from file: %s ...\n", path)
	}
	if _, err := client.SubmitConnector(ctx, string(payload)); err != nil {
		return fmt.Errorf("failed to submit connector: %w", err)
	}

	if jsonMode {
		printActionJSON(meta.Name, "create")
		return nil
	}
	color.Green("Connector submitted successfully!\n")
	return nil
}

func configureConnector(cmd *cobra.Command, name string, connectorConfig map[string]string, required []string, passwordField string) error {
	color.Yellow("\n  Starting configuration for %s...\n", name)

	questions := make([]*survey.Question, 0, len(required))
	for _, field := range required {
		var prompt survey.Prompt
		if field == passwordField {
			prompt = &survey.Password{Message: fmt.Sprintf("Enter %s:", field)}
		} else {
			prompt = &survey.Input{Message: fmt.Sprintf("Enter %s:", field)}
		}
		questions = append(questions, &survey.Question{
			Name:     field,
			Prompt:   prompt,
			Validate: survey.Required,
		})
	}

	answers := make(map[string]interface{})
	if err := survey.Ask(questions, &answers); err != nil {
		if util.IsSurveyInterrupt(err) {
			return util.ErrCanceled
		}
		return fmt.Errorf("failed to get input: %w", err)
	}

	for key, value := range answers {
		connectorConfig[key] = fmt.Sprintf("%v", value)
	}

	client, err := util.NewKafkaConnectClient()
	if err != nil {
		return err
	}

	return reviewAndSubmit(cmd, client, name, connectorConfig)
}

// reviewAndSubmit runs the shared tail of the create wizard: an optional
// change-any-field loop, the final config display, and (after confirmation
// and server-side validation) the submission. With --dry-run the config is
// validated and displayed but not submitted.
func reviewAndSubmit(cmd *cobra.Command, client *connector.Client, name string, connectorConfig map[string]string) error {
	ctx := cmd.Context()

	if err := promptSchemaRegistryFormat(connectorConfig); err != nil {
		return err
	}

	for {
		finalConfig, err := util.ToPrettyJSON(connectorConfig)
		if err != nil {
			return fmt.Errorf("failed to format config: %w", err)
		}
		color.Cyan("\n Current %s Configuration:\n", name)
		fmt.Println(finalConfig)

		var confirmChange bool
		changePrompt := &survey.Confirm{
			Message: "Do you want to change any field?",
			Default: false,
		}
		if err := survey.AskOne(changePrompt, &confirmChange); err != nil {
			if util.IsSurveyInterrupt(err) {
				return util.ErrCanceled
			}
			return fmt.Errorf("prompt failed: %w", err)
		}

		if !confirmChange {
			color.Green("\n Configuration complete!\n")
			break
		}

		var fieldToChange string
		fieldPrompt := &survey.Select{
			Message: "Which field do you want to change?",
			Options: util.KeysFromMap(connectorConfig),
		}
		if err := survey.AskOne(fieldPrompt, &fieldToChange); err != nil {
			if util.IsSurveyInterrupt(err) {
				return util.ErrCanceled
			}
			return fmt.Errorf("prompt failed: %w", err)
		}

		var newValue string
		valuePrompt := &survey.Input{
			Message: fmt.Sprintf("Enter new value for %s:", fieldToChange),
		}
		if err := survey.AskOne(valuePrompt, &newValue); err != nil {
			if util.IsSurveyInterrupt(err) {
				return util.ErrCanceled
			}
			return fmt.Errorf("prompt failed: %w", err)
		}

		connectorConfig[fieldToChange] = newValue
	}

	// POST /connectors requires a connector name; templates don't carry one.
	if connectorConfig["name"] == "" {
		var connName string
		if err := survey.AskOne(&survey.Input{Message: "Connector name:"}, &connName, survey.WithValidator(survey.Required)); err != nil {
			return util.ErrCanceled
		}
		connectorConfig["name"] = connName
	}

	finalConfig, err := util.ToPrettyJSON(connectorConfig)
	if err != nil {
		return fmt.Errorf("failed to format config: %w", err)
	}
	color.Cyan("\nFinal %s Configuration:\n", name)
	fmt.Println(finalConfig)

	if util.IsDryRun(cmd) {
		if !validateConfigOrConfirm(ctx, client, connectorConfig) {
			color.Yellow("\n Submission cancelled. Exiting.\n")
			return util.ErrNothingToDo
		}
		color.Yellow("[dry-run] Would submit connector %s\n", name)
		return nil
	}

	var submitConfirm bool
	submitPrompt := &survey.Confirm{
		Message: "Do you want to submit this connector to Kafka Connect?",
		Default: true,
	}
	if err := survey.AskOne(submitPrompt, &submitConfirm); err != nil {
		if util.IsSurveyInterrupt(err) {
			return util.ErrCanceled
		}
		return fmt.Errorf("prompt failed: %w", err)
	}

	if !submitConfirm {
		color.Yellow("\n Submission cancelled. Exiting.\n")
		return util.ErrNothingToDo
	}

	color.Green("\n Submitting connector...\n")
	if !validateConfigOrConfirm(ctx, client, connectorConfig) {
		color.Yellow("\n Submission cancelled. Exiting.\n")
		return util.ErrNothingToDo
	}

	// POST /connectors expects {"name": ..., "config": {...}}.
	payload, err := json.Marshal(map[string]any{
		"name":   connectorConfig["name"],
		"config": connectorConfig,
	})
	if err != nil {
		return fmt.Errorf("failed to encode config: %w", err)
	}
	if _, err := client.SubmitConnector(ctx, string(payload)); err != nil {
		return fmt.Errorf("failed to submit connector: %w", err)
	}
	color.Green("Connector submitted successfully!\n")
	return nil
}

// configureFromPlugin builds a connector config for any plugin installed on
// the cluster, using the validate endpoint to discover which fields must be
// filled in. class may be empty, in which case the user picks a plugin
// interactively.
func configureFromPlugin(cmd *cobra.Command, class string) error {
	ctx := cmd.Context()

	client, err := util.NewKafkaConnectClient()
	if err != nil {
		return err
	}

	stop := util.StartSpinner("Fetching connector plugins...")
	plugins, err := client.ListConnectorPlugins(ctx)
	stop()
	if err != nil {
		return fmt.Errorf("failed to list connector plugins: %w", err)
	}
	if len(plugins) == 0 {
		color.Yellow("No connector plugins found\n")
		return util.ErrNothingToDo
	}

	var plugin connector.Plugin
	if class != "" {
		found := false
		for _, p := range plugins {
			if p.Class == class || strings.HasSuffix(p.Class, "."+class) {
				plugin = p
				found = true
				break
			}
		}
		// The plugins endpoint hides some loadable classes (e.g. bundled
		// tools), so an unlisted class is a warning, not an error — the
		// validation rounds below will catch a class that truly doesn't exist.
		if !found {
			color.Yellow("Plugin class %s is not in the cluster's plugin list; proceeding anyway\n", class)
			plugin = connector.Plugin{Class: class}
		}
	} else {
		options := make([]string, 0, len(plugins)+1)
		for _, p := range plugins {
			version := p.Version
			if version == "" {
				version = "unknown"
			}
			options = append(options, fmt.Sprintf("%s  (%s %s)", p.Class, p.Type, version))
		}
		const cancelOpt = "← Cancel"
		options = append(options, cancelOpt)

		var answer survey.OptionAnswer
		if err := survey.AskOne(&survey.Select{
			Message:  "Pick a connector plugin:",
			Options:  options,
			PageSize: 10,
		}, &answer); err != nil || answer.Index >= len(plugins) {
			return util.ErrCanceled
		}
		plugin = plugins[answer.Index]
	}

	var name string
	if err := survey.AskOne(&survey.Input{Message: "Connector name:"}, &name, survey.WithValidator(survey.Required)); err != nil {
		return util.ErrCanceled
	}

	cfg := map[string]string{
		"connector.class": plugin.Class,
		"name":            name,
		"tasks.max":       "1",
	}
	if strings.EqualFold(plugin.Type, "sink") {
		var topics string
		if err := survey.AskOne(&survey.Input{
			Message: "Topics to consume (comma-separated):",
		}, &topics, survey.WithValidator(survey.Required)); err != nil {
			return util.ErrCanceled
		}
		cfg["topics"] = topics
	}

	// Iteratively validate and prompt for every required field that doesn't
	// have an explicit value yet (its default, if any, is pre-filled as the
	// suggested answer), plus any field the server reports as currently
	// invalid.
	const maxRounds = 3
	for range maxRounds {
		stop := util.StartSpinner("Validating config...")
		res, err := client.ValidateConnectorConfig(ctx, plugin.Class, cfg)
		stop()
		if err != nil {
			color.Yellow("Could not validate config: %v\n", err)
			break
		}

		fields := fieldsToPrompt(res)
		if len(fields) == 0 {
			break
		}

		color.Cyan("\n%d field(s) need attention:\n", len(fields))
		for _, f := range fields {
			if err := promptForField(f, cfg); err != nil {
				return err
			}
		}
	}

	return reviewAndSubmit(cmd, client, plugin.Class, cfg)
}

// promptField is a config field the plugin wizard should ask the user about.
type promptField struct {
	Name        string
	Doc         string
	Default     string
	Recommended []string
	Errors      []string
	Secret      bool
}

// fieldsToPrompt selects the config fields worth prompting for: every
// required field without an explicit value yet (regardless of whether the
// plugin defines a default — the default is surfaced as the suggested answer
// rather than silently accepted), plus any field the server reported errors
// for. connector.class and name are managed by the wizard itself.
func fieldsToPrompt(res connector.ConfigValidationResponse) []promptField {
	var fields []promptField
	for _, c := range res.Configs {
		name := c.Definition.Name
		if name == "" {
			name = c.Value.Name
		}
		if name == "" || name == "connector.class" || name == "name" {
			continue
		}

		value := anyToString(c.Value.Value)
		def := anyToString(c.Definition.DefaultValue)
		needsInput := c.Definition.Required && value == ""
		if !needsInput && len(c.Value.Errors) == 0 {
			continue
		}

		f := promptField{
			Name:    name,
			Doc:     c.Definition.Documentation,
			Default: value,
			Errors:  c.Value.Errors,
			Secret:  isSecretField(name),
		}
		if f.Default == "" {
			f.Default = def
		}
		for _, rv := range c.Value.RecommendedValues {
			f.Recommended = append(f.Recommended, anyToString(rv))
		}
		fields = append(fields, f)
	}
	return fields
}

// promptForField asks the user for one field's value and stores a non-empty
// answer in cfg. Empty answers leave the field unset so the next validation
// round (or the pre-submit gate) can flag it again.
func promptForField(f promptField, cfg map[string]string) error {
	for _, e := range f.Errors {
		color.Red("  %s\n", e)
	}

	msg := fmt.Sprintf("Enter %s:", f.Name)
	var value string
	var prompt survey.Prompt
	switch {
	case f.Secret:
		prompt = &survey.Password{Message: msg, Help: f.Doc}
	case len(f.Recommended) > 0:
		prompt = &survey.Select{Message: msg, Options: f.Recommended, Help: f.Doc, Default: selectDefault(f.Recommended, f.Default)}
	default:
		prompt = &survey.Input{Message: msg, Default: f.Default, Help: f.Doc}
	}
	if err := survey.AskOne(prompt, &value); err != nil {
		if util.IsSurveyInterrupt(err) {
			return util.ErrCanceled
		}
		return fmt.Errorf("prompt failed: %w", err)
	}
	if value != "" {
		cfg[f.Name] = value
	}
	return nil
}

// selectDefault returns def if it is one of the options, so survey.Select
// does not fail on a default that is not in the list.
func selectDefault(options []string, def string) any {
	if slices.Contains(options, def) {
		return def
	}
	return nil
}

// anyToString renders a typed validate-endpoint value ("", number, bool, or
// null) as the string Kafka Connect expects in a connector config.
func anyToString(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%v", v)
}

// secretFieldMarkers are substrings of config keys that hold credentials.
var secretFieldMarkers = []string{"password", "secret", "token", "credential", "jaas.config", "api.key", "apikey", "private.key"}

// isSecretField reports whether a config key looks like a credential, so the
// wizard can mask the input.
func isSecretField(name string) bool {
	n := strings.ToLower(name)
	for _, m := range secretFieldMarkers {
		if strings.Contains(n, m) {
			return true
		}
	}
	return false
}
