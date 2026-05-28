package connector

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"gokafkaconnect/internal/connector"
	template "gokafkaconnect/internal/connector/kafka/templates"
	"gokafkaconnect/internal/util"

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

var connectorJSONPath string

// CreateCmd represents the create command.
var CreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a connector from predefined configuration",
	Long:  `Browse predefined connector.`,
	Run: func(cmd *cobra.Command, _ []string) {
		if connectorJSONPath != "" {
			submitConnectorFromFile(cmd.Context(), connectorJSONPath)
			return
		}

		var selected string
		color.Cyan("\n Available Kafka Connectors \n")
		prompt := &survey.Select{
			Message: "Pick a connector to work with:",
			Options: connectors,
		}
		err := survey.AskOne(prompt, &selected)
		if err != nil {
			color.Yellow("Canceled\n")
			return
		}
		color.Green("\n You selected: %s\n", selected)
		switch selected {
		case "RabbitMQ Connector":
			configureConnector(cmd.Context(), "RabbitMQ Connector", template.GetRabbitMQConnectorTemplate(), template.RabbitMQRequiredFields(), "rabbitmq.password")
		case "Debezium PostgreSQL CDC":
			configureConnector(cmd.Context(), "Debezium PostgreSQL CDC", template.GetDebeziumPostgresConnectorTemplate(), template.DebeziumPostgresRequiredFields(), "database.password")
		case "JDBC Source Connector":
			configureConnector(cmd.Context(), "JDBC Source Connector", template.GetJDBCSourceConnectorTemplate(), template.JDBCSourceRequiredFields(), "connection.password")
		case "JDBC Sink Connector":
			configureConnector(cmd.Context(), "JDBC Sink Connector", template.GetJDBCSinkConnectorTemplate(), template.JDBCSinkRequiredFields(), "connection.password")
		case "S3 Sink Connector":
			configureConnector(cmd.Context(), "S3 Sink Connector", template.GetS3SinkConnectorTemplate(), template.S3SinkRequiredFields(), "")
		}
	},
}

func init() {
	CreateCmd.Flags().StringVarP(&connectorJSONPath, "file", "f", "", "Path to connector JSON config file")
}

func submitConnectorFromFile(ctx context.Context, path string) {
	b, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		color.Red("Failed to read file %s: %v\n", path, err)
		return
	}

	var js json.RawMessage
	if err := json.Unmarshal(b, &js); err != nil {
		color.Red("Invalid JSON in %s: %v\n", path, err)
		return
	}

	cfg, err := util.LoadConfig()
	if err != nil {
		color.Red("Failed to load config file: %v\n", err)
		return
	}

	client := connector.NewClient(cfg.KafkaConnect.URL)
	if cfg.KafkaConnect.Username != "" {
		client.SetBasicAuth(cfg.KafkaConnect.Username, cfg.KafkaConnect.Password)
	}

	color.Green("\n Submitting connector from file: %s ...\n", path)
	if _, err := client.SubmitConnector(ctx, string(b)); err != nil {
		color.Red("Failed to submit connector: %v\n", err)
		return
	}
	color.Green("Connector submitted successfully!\n")
}

func configureConnector(ctx context.Context, name string, connectorConfig map[string]string, required []string, passwordField string) {
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
	err := survey.Ask(questions, &answers)
	if err != nil {
		if util.IsSurveyInterrupt(err) {
			color.Yellow("Canceled\n")
		} else {
			color.Red("Failed to get input: %v\n", err)
		}
		return
	}

	for key, value := range answers {
		connectorConfig[key] = fmt.Sprintf("%v", value)
	}

	for {
		finalConfig, err := util.ToPrettyJSON(connectorConfig)
		if err != nil {
			color.Red("Failed to format config: %v\n", err)
			return
		}
		color.Cyan("\n Current %s Configuration:\n", name)
		fmt.Println(finalConfig)

		var confirmChange bool
		changePrompt := &survey.Confirm{
			Message: "Do you want to change any field?",
			Default: false,
		}
		err = survey.AskOne(changePrompt, &confirmChange)
		if err != nil {
			if util.IsSurveyInterrupt(err) {
				color.Yellow("Canceled\n")
			} else {
				color.Red("Prompt failed: %v\n", err)
			}
			return
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
		err = survey.AskOne(fieldPrompt, &fieldToChange)
		if err != nil {
			if util.IsSurveyInterrupt(err) {
				color.Yellow("Canceled\n")
			} else {
				color.Red("Prompt failed: %v\n", err)
			}
			return
		}

		var newValue string
		valuePrompt := &survey.Input{
			Message: fmt.Sprintf("Enter new value for %s:", fieldToChange),
		}
		err = survey.AskOne(valuePrompt, &newValue)
		if err != nil {
			if util.IsSurveyInterrupt(err) {
				color.Yellow("Canceled\n")
			} else {
				color.Red("Prompt failed: %v\n", err)
			}
			return
		}

		connectorConfig[fieldToChange] = newValue
	}
	finalConfig, err := util.ToPrettyJSON(connectorConfig)
	if err != nil {
		color.Red("Failed to format config: %v\n", err)
		return
	}
	color.Cyan("\nFinal %s Configuration:\n", name)
	fmt.Println(finalConfig)

	var submitConfirm bool
	submitPrompt := &survey.Confirm{
		Message: "Do you want to submit this connector to Kafka Connect?",
		Default: true,
	}
	err = survey.AskOne(submitPrompt, &submitConfirm)
	if err != nil {
		if util.IsSurveyInterrupt(err) {
			color.Yellow("Canceled\n")
		} else {
			color.Red("Prompt failed: %v\n", err)
		}
		return
	}

	if submitConfirm {
		color.Green("\n Submitting connector...\n")
		cfg, err := util.LoadConfig()

		if err != nil {
			color.Red("Failed to load config file: %v\n", err)
			return
		}
		client := connector.NewClient(cfg.KafkaConnect.URL)
		if cfg.KafkaConnect.Username != "" {
			client.SetBasicAuth(cfg.KafkaConnect.Username, cfg.KafkaConnect.Password)
		}

		_, err = client.SubmitConnector(ctx, finalConfig)
		if err != nil {
			color.Red("Failed to submit connector: %v\n", err)
		} else {
			color.Green("Connector submitted successfully!\n")
		}
	} else {
		color.Yellow("\n Submission cancelled. Exiting.\n")
	}
}
