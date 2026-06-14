package util

import (
	"context"
	"fmt"
	"strconv"

	"github.com/Maksim-Gr/kkon/internal/connector"

	"github.com/AlecAivazis/survey/v2"
	"github.com/fatih/color"
)

// NewKafkaConnectClient creates a connector client using the configured Kafka Connect URL.
func NewKafkaConnectClient() (*connector.Client, bool) {
	cfg, err := LoadConfig()
	if err != nil {
		color.Red("Failed to load config: %v\n", err)
		return nil, false
	}
	client := connector.NewClient(cfg.KafkaConnect.URL)
	if cfg.KafkaConnect.Username != "" {
		client.SetBasicAuth(cfg.KafkaConnect.Username, cfg.KafkaConnect.Password)
	}
	return client, true
}

// ResolveConnectorName returns a connector name from:
//  1. provided flag value (if not empty), or
//  2. interactive selection from the API.
func ResolveConnectorName(ctx context.Context, client *connector.Client, flagValue string) (string, bool) {
	if flagValue != "" {
		return flagValue, true
	}

	connectors, err := client.ListConnectors(ctx)
	if err != nil {
		color.Red("Failed to list connectors: %v\n", err)
		return "", false
	}
	if len(connectors) == 0 {
		color.Yellow("No connectors found\n")
		return "", false
	}

	const cancelOpt = "← Cancel"
	var name string
	prompt := &survey.Select{Message: "Pick connector:", Options: append(connectors, cancelOpt)}
	if err := survey.AskOne(prompt, &name); err != nil || name == cancelOpt {
		color.Yellow("Canceled\n")
		return "", false
	}
	return name, true
}

// ResolveTaskID returns a task id from:
//  1. provided flag value (if >= 0), or
//  2. interactive selection from the API.
func ResolveTaskID(ctx context.Context, client *connector.Client, connectorName string, flagValue int, dryRun bool) (int, bool) {
	if flagValue >= 0 {
		if dryRun {
			return flagValue, true
		}
		tasks, err := client.ListConnectorTasks(ctx, connectorName)
		if err != nil {
			color.Red("Failed to list tasks for %s: %v\n", connectorName, err)
			return -1, false
		}
		for _, t := range tasks {
			if t.Task == flagValue {
				return flagValue, true
			}
		}
		color.Red("Task %d not found for connector %s\n", flagValue, connectorName)
		return -1, false
	}

	if dryRun {
		color.Yellow("[dry-run] Would ask for task id for connector: %s\n", connectorName)
		return -1, false
	}

	tasks, err := client.ListConnectorTasks(ctx, connectorName)
	if err != nil {
		color.Red("Failed to list tasks for %s: %v\n", connectorName, err)
		return -1, false
	}
	if len(tasks) == 0 {
		color.Yellow("No tasks found for %s\n", connectorName)
		return -1, false
	}

	const cancelOpt = "← Cancel"
	options := make([]string, 0, len(tasks)+1)
	for _, t := range tasks {
		options = append(options, strconv.Itoa(t.Task))
	}
	options = append(options, cancelOpt)

	var selected string
	prompt := &survey.Select{Message: "Pick task id:", Options: options}
	if err := survey.AskOne(prompt, &selected); err != nil || selected == cancelOpt {
		color.Yellow("Canceled\n")
		return -1, false
	}

	id, err := strconv.Atoi(selected)
	if err != nil {
		color.Red("Invalid task id: %v\n", err)
		return -1, false
	}
	return id, true
}

// FormatTaskRef returns a human-readable "connector task N" string.
func FormatTaskRef(connectorName string, taskID int) string {
	return fmt.Sprintf("%s task %d", connectorName, taskID)
}
