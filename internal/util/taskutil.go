package util

import (
	"context"
	"fmt"
	"sort"
	"strconv"

	"github.com/Maksim-Gr/kkon/internal/connector"
	"github.com/Maksim-Gr/kkon/internal/prompt"

	"github.com/fatih/color"
)

// NewKafkaConnectClient creates a connector client using the configured Kafka Connect URL.
func NewKafkaConnectClient() (*connector.Client, error) {
	cfg, err := ResolveConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	client := connector.NewClient(cfg.KafkaConnect.URL)
	if cfg.KafkaConnect.Username != "" {
		client.SetBasicAuth(cfg.KafkaConnect.Username, cfg.KafkaConnect.Password)
	}
	return client, nil
}

// ResolveConnectorName returns a connector name from:
//  1. provided flag value (if not empty), or
//  2. interactive selection from the API.
//
// It returns ErrCanceled if the user cancels the selection and ErrNothingToDo
// if there are no connectors to choose from.
func ResolveConnectorName(ctx context.Context, client *connector.Client, flagValue string) (string, error) {
	if flagValue != "" {
		return flagValue, nil
	}

	connectors, err := client.ListConnectors(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to list connectors: %w", err)
	}
	if len(connectors) == 0 {
		color.Yellow("No connectors found\n")
		return "", ErrNothingToDo
	}

	const cancelOpt = "← Cancel"
	i, err := prompt.Select("Pick connector:", append(connectors, cancelOpt), prompt.Options{})
	if err != nil {
		return "", err
	}
	if i == len(connectors) {
		return "", ErrCanceled
	}
	return connectors[i], nil
}

// ResolveConnectorNames returns connector names from:
//  1. positional args (if any), or
//  2. all connectors when all is true, or
//  3. interactive multi-selection from the API.
//
// It returns ErrCanceled if the user cancels the selection and ErrNothingToDo
// if there are no connectors to choose from.
func ResolveConnectorNames(ctx context.Context, client *connector.Client, args []string, all bool) ([]string, error) {
	if len(args) > 0 && all {
		return nil, fmt.Errorf("cannot combine --all with connector names")
	}
	if len(args) > 0 {
		return args, nil
	}

	connectors, err := client.ListConnectors(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list connectors: %w", err)
	}
	if len(connectors) == 0 {
		color.Yellow("No connectors found\n")
		return nil, ErrNothingToDo
	}
	sort.Strings(connectors)

	if all {
		return connectors, nil
	}

	idx, err := prompt.MultiSelect("Pick connectors:", connectors)
	if err != nil {
		return nil, err
	}
	selected := make([]string, len(idx))
	for i, j := range idx {
		selected[i] = connectors[j]
	}
	return selected, nil
}

// ResolveTaskID returns a task id from:
//  1. provided flag value (if >= 0), or
//  2. interactive selection from the API.
//
// It returns ErrCanceled if the user cancels the selection and ErrNothingToDo
// if the connector has no tasks.
func ResolveTaskID(ctx context.Context, client *connector.Client, connectorName string, flagValue int, dryRun bool) (int, error) {
	if flagValue >= 0 {
		if dryRun {
			return flagValue, nil
		}
		tasks, err := client.ListConnectorTasks(ctx, connectorName)
		if err != nil {
			return -1, fmt.Errorf("failed to list tasks for %s: %w", connectorName, err)
		}
		for _, t := range tasks {
			if t.Task == flagValue {
				return flagValue, nil
			}
		}
		return -1, fmt.Errorf("task %d not found for connector %s", flagValue, connectorName)
	}

	if dryRun {
		color.Yellow("[dry-run] Would ask for task id for connector: %s\n", connectorName)
		return -1, ErrNothingToDo
	}

	tasks, err := client.ListConnectorTasks(ctx, connectorName)
	if err != nil {
		return -1, fmt.Errorf("failed to list tasks for %s: %w", connectorName, err)
	}
	if len(tasks) == 0 {
		color.Yellow("No tasks found for %s\n", connectorName)
		return -1, ErrNothingToDo
	}

	const cancelOpt = "← Cancel"
	options := make([]string, 0, len(tasks)+1)
	for _, t := range tasks {
		options = append(options, strconv.Itoa(t.Task))
	}
	options = append(options, cancelOpt)

	i, err := prompt.Select("Pick task id:", options, prompt.Options{})
	if err != nil {
		return -1, err
	}
	if i == len(tasks) {
		return -1, ErrCanceled
	}
	return tasks[i].Task, nil
}

// FormatTaskRef returns a human-readable "connector task N" string.
func FormatTaskRef(connectorName string, taskID int) string {
	return fmt.Sprintf("%s task %d", connectorName, taskID)
}
