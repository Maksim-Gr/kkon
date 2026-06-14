// Package tests contains integration tests for the Kafka Connect client.
package tests

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	c "github.com/Maksim-Gr/kkon/internal/connector"

	"github.com/stretchr/testify/require"
)

const mockSinkConfigTemplate = `{
	"name": "%s",
	"config": {
		"connector.class": "org.apache.kafka.connect.tools.MockSinkConnector",
		"tasks.max": "1",
		"topics": "%s"
	}
}`

// setupConnectors creates a set of connectors for testing.
func setupConnectors(t *testing.T, client *c.Client, names []string) {
	t.Helper()

	for i, name := range names {
		topic := fmt.Sprintf("test-topic-%c", 'A'+i)
		config := fmt.Sprintf(mockSinkConfigTemplate, name, topic)

		_, err := client.SubmitConnector(context.Background(), config)
		require.NoError(t, err, "creating connector: %s", name)
	}
}

func cleanupConnectors(t *testing.T, client *c.Client, names []string) {
	t.Helper()
	for _, name := range names {
		if err := client.DeleteConnector(context.Background(), name); err != nil {
			t.Logf("cleanup: failed to delete connector %s: %v", name, err)
		}
	}
}

func TestConnectorLifecycle(t *testing.T) {
	kc := KafkaConnectFixture(t)
	client := c.NewClient(kc.URL)

	connectorNames := []string{
		"test-op-connector-1",
		"test-op-connector-2",
		"test-op-connector-3",
	}

	cleanupConnectors(t, client, connectorNames)
	defer cleanupConnectors(t, client, connectorNames)

	t.Run("CreateAndList", func(t *testing.T) {
		setupConnectors(t, client, connectorNames)

		got, err := client.ListConnectors(context.Background())
		require.NoError(t, err)

		for _, name := range connectorNames {
			require.Contains(t, got, name)
		}
	})

	t.Run("ListStatuses", func(t *testing.T) {
		var statuses c.ConnectorsStatusResponse
		require.Eventually(t, func() bool {
			var err error
			statuses, err = client.ListConnectorStatuses(context.Background())
			if err != nil {
				return false
			}
			for _, name := range connectorNames {
				if _, ok := statuses[name]; !ok {
					return false
				}
			}
			return true
		}, 10*time.Second, 500*time.Millisecond, "connector statuses did not propagate in time")

		for _, name := range connectorNames {
			_, ok := statuses[name]
			require.True(t, ok, "status missing for connector: %s", name)
		}
	})

	t.Run("BackupConnectorConfig", func(t *testing.T) {
		outputDir := os.TempDir()

		backupFile, err := c.BackupConnectorConfig(context.Background(), client, connectorNames[:2], outputDir)
		require.NoError(t, err, "BackupConnectorConfig should not return an error")

		require.FileExists(t, backupFile, "Backup file should exist")
		require.Contains(t, filepath.Base(backupFile), "config_", "Backup file name should contain 'config_' prefix")

		defer func() {
			if err := os.Remove(backupFile); err != nil && !os.IsNotExist(err) {
				t.Logf("failed to remove backup file: %v", err)
			}
		}()
	})

	t.Run("DeleteOne", func(t *testing.T) {
		target := connectorNames[2]

		err := client.DeleteConnector(context.Background(), target)
		require.NoError(t, err)

		got, err := client.ListConnectors(context.Background())
		require.NoError(t, err)
		require.NotContains(t, got, target)
	})

	t.Run("DeleteNonExistentReturnsError", func(t *testing.T) {
		err := client.DeleteConnector(context.Background(), "non-existent-connector")
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to delete connector")
	})

	t.Run("GetConnectorStatus", func(t *testing.T) {
		target := connectorNames[0]
		var s c.Status
		require.Eventually(t, func() bool {
			var err error
			s, err = client.GetConnectorStatus(context.Background(), target)
			return err == nil && s.Connector.State != ""
		}, 10*time.Second, 500*time.Millisecond, "connector status did not propagate")
		require.Equal(t, target, s.Name)
		require.NotEmpty(t, s.Connector.State)
	})

	t.Run("ListConnectorTasks", func(t *testing.T) {
		target := connectorNames[0]
		var tasks []c.TaskRef
		require.Eventually(t, func() bool {
			var err error
			tasks, err = client.ListConnectorTasks(context.Background(), target)
			return err == nil && len(tasks) > 0
		}, 10*time.Second, 500*time.Millisecond, "tasks did not appear in time")
		require.Equal(t, target, tasks[0].Connector)
	})

	t.Run("RestartConnectorTask", func(t *testing.T) {
		target := connectorNames[0]
		tasks, err := client.ListConnectorTasks(context.Background(), target)
		require.NoError(t, err)
		require.NotEmpty(t, tasks)
		err = client.RestartConnectorTask(context.Background(), target, tasks[0].Task)
		require.NoError(t, err)
		require.Eventually(t, func() bool {
			s, err := client.GetConnectorStatus(context.Background(), target)
			return err == nil && len(s.Tasks) > 0
		}, 10*time.Second, 500*time.Millisecond, "task did not recover after restart")
	})
}

func TestGetConnectorConfig(t *testing.T) {
	kc := KafkaConnectFixture(t)
	client := c.NewClient(kc.URL)

	connectorName := "test-getconfig"
	topic := "topic-gc"
	defer cleanupConnectors(t, client, []string{connectorName})

	config := fmt.Sprintf(mockSinkConfigTemplate, connectorName, topic)

	_, err := client.SubmitConnector(context.Background(), config)
	require.NoError(t, err)

	jsonConfig, err := client.GetConnectorConfig(context.Background(), connectorName)
	require.NoError(t, err)

	require.Contains(t, jsonConfig, "connector.class")
	require.Contains(t, jsonConfig, "MockSinkConnector")
}

func TestSubmitAndList(t *testing.T) {
	kc := KafkaConnectFixture(t)
	client := c.NewClient(kc.URL)

	connectorName := "test-submit-list"
	topic := "test-topic-sl"
	defer cleanupConnectors(t, client, []string{connectorName})

	config := fmt.Sprintf(mockSinkConfigTemplate, connectorName, topic)

	_, err := client.SubmitConnector(context.Background(), config)
	require.NoError(t, err)

	got, err := client.ListConnectors(context.Background())
	require.NoError(t, err)
	require.Contains(t, got, connectorName)
}
