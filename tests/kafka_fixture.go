package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	confluentVersion  = "7.5.0"
	zookeeperImage    = "confluentinc/cp-zookeeper:" + confluentVersion
	kafkaImage        = "confluentinc/cp-kafka:" + confluentVersion
	kafkaConnectImage = "confluentinc/cp-kafka-connect:" + confluentVersion
)

// WaitForKafkaConnectStartUp polls the /connector-plugins endpoint to ensure Kafka Connect is fully initialized.
func WaitForKafkaConnectStartUp(t *testing.T, baseURL string, timeout time.Duration) {
	t.Helper()

	type Plugin struct {
		Class string `json:"class"`
	}

	require.Eventually(t, func() bool {
		resp, err := http.Get(fmt.Sprintf("%s/connector-plugins", baseURL))
		if err != nil || resp.StatusCode != http.StatusOK {
			return false
		}
		defer resp.Body.Close() //nolint:errcheck
		var plugins []Plugin
		return json.NewDecoder(resp.Body).Decode(&plugins) == nil && len(plugins) > 0
	}, timeout, 2*time.Second, "Kafka Connect did not become ready within %s", timeout)
}

// KafkaConnectTestFixture holds a running Kafka Connect container and its URL.
type KafkaConnectTestFixture struct {
	Container testcontainers.Container
	URL       string
	Network   string
}

// KafkaConnectFixture starts a full Kafka + Kafka Connect stack and returns a test fixture.
func KafkaConnectFixture(t *testing.T) *KafkaConnectTestFixture {
	t.Helper()
	if testing.Short() {
		require.NotEqual(t, "1", os.Getenv("KKON_REQUIRE_DOCKER"), "strict integration tests must not use -short")
		t.Skip("skipping Kafka Connect integration test in -short mode")
	}
	if os.Getenv("KKON_REQUIRE_DOCKER") == "1" {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		provider, err := testcontainers.ProviderDocker.GetProvider()
		require.NoError(t, err, "Docker is required by KKON_REQUIRE_DOCKER=1")
		require.NoError(t, provider.Health(ctx), "Docker is required by KKON_REQUIRE_DOCKER=1")
	} else {
		testcontainers.SkipIfProviderIsNotHealthy(t)
	}
	ctx := context.Background()

	nw, err := network.New(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = nw.Remove(ctx) })

	// Zookeeper Setup
	zooReq := testcontainers.ContainerRequest{
		Image:        zookeeperImage,
		ExposedPorts: []string{"2181/tcp"},
		Networks:     []string{nw.Name},
		Env: map[string]string{
			"ZOOKEEPER_CLIENT_PORT": "2181",
			"ZOOKEEPER_TICK_TIME":   "2000",
		},
		WaitingFor:     wait.ForListeningPort("2181/tcp").WithStartupTimeout(30 * time.Second),
		NetworkAliases: map[string][]string{nw.Name: {"zookeeper"}},
	}
	zooC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: zooReq,
		Started:          true,
	})
	cleanupKafkaContainer(t, "zookeeper", zooC)
	require.NoError(t, err)

	// Kafka Broker Setup
	kafkaReq := testcontainers.ContainerRequest{
		Image:        kafkaImage,
		ExposedPorts: []string{"9092/tcp"},
		Networks:     []string{nw.Name},
		Env: map[string]string{
			"KAFKA_BROKER_ID":                        "1",
			"KAFKA_ZOOKEEPER_CONNECT":                "zookeeper:2181",
			"KAFKA_ADVERTISED_LISTENERS":             "PLAINTEXT://kafka:9092",
			"KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR": "1",
			"KAFKA_LISTENERS":                        "PLAINTEXT://0.0.0.0:9092",
			"KAFKA_LOG_DIRS":                         "/tmp/kafka-logs",
		},
		WaitingFor:     wait.ForListeningPort("9092/tcp").WithStartupTimeout(90 * time.Second),
		NetworkAliases: map[string][]string{nw.Name: {"kafka"}},
	}
	kafkaC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: kafkaReq,
		Started:          true,
	})
	cleanupKafkaContainer(t, "kafka", kafkaC)
	require.NoError(t, err)

	// Kafka Connect Setup
	connectReq := testcontainers.ContainerRequest{
		Image:        kafkaConnectImage,
		ExposedPorts: []string{"8083/tcp"},
		Networks:     []string{nw.Name},
		Env: map[string]string{
			"CONNECT_BOOTSTRAP_SERVERS":                 "kafka:9092",
			"CONNECT_REST_PORT":                         "8083",
			"CONNECT_GROUP_ID":                          "quickstart",
			"CONNECT_CONFIG_STORAGE_TOPIC":              "docker-connect-configs",
			"CONNECT_OFFSET_STORAGE_TOPIC":              "docker-connect-offsets",
			"CONNECT_STATUS_STORAGE_TOPIC":              "docker-connect-status",
			"CONNECT_CONFIG_STORAGE_REPLICATION_FACTOR": "1",
			"CONNECT_OFFSET_STORAGE_REPLICATION_FACTOR": "1",
			"CONNECT_STATUS_STORAGE_REPLICATION_FACTOR": "1",
			"CONNECT_KEY_CONVERTER":                     "org.apache.kafka.connect.json.JsonConverter",
			"CONNECT_VALUE_CONVERTER":                   "org.apache.kafka.connect.json.JsonConverter",
			"CONNECT_INTERNAL_KEY_CONVERTER":            "org.apache.kafka.connect.json.JsonConverter",
			"CONNECT_INTERNAL_VALUE_CONVERTER":          "org.apache.kafka.connect.json.JsonConverter",
			"CONNECT_LOG4J_ROOT_LOGLEVEL":               "INFO",
			"CONNECT_PLUGIN_PATH":                       "/usr/share/java,/usr/share/confluent-hub-components",
			"CONNECT_REST_ADVERTISED_HOST_NAME":         "localhost",
		},
		WaitingFor:     wait.ForHTTP("/").WithPort("8083/tcp").WithStartupTimeout(120 * time.Second),
		NetworkAliases: map[string][]string{nw.Name: {"kafkaconnect"}},
	}
	connectC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: connectReq,
		Started:          true,
	})

	cleanupKafkaContainer(t, "kafka-connect", connectC)
	require.NoError(t, err)

	connectHost, err := connectC.Host(ctx)
	require.NoError(t, err)
	connectPort, err := connectC.MappedPort(ctx, "8083")
	require.NoError(t, err)

	connectURL := fmt.Sprintf("http://%s:%s", connectHost, connectPort.Port())
	WaitForKafkaConnectStartUp(t, connectURL, 20*time.Second)

	return &KafkaConnectTestFixture{ // Return pointer
		Container: connectC,
		URL:       connectURL,
		Network:   nw.Name,
	}
}

// cleanupKafkaContainer records failure diagnostics before terminating the container.
func cleanupKafkaContainer(t *testing.T, name string, container testcontainers.Container) {
	t.Helper()
	if container == nil {
		return
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if t.Failed() {
			logs, err := container.Logs(ctx)
			if err != nil {
				t.Logf("%s: cannot read container logs: %v", name, err)
			} else {
				data, readErr := io.ReadAll(logs)
				_ = logs.Close()
				t.Logf("%s logs:\n%s", name, data)
				if readErr != nil {
					t.Logf("%s: incomplete container logs: %v", name, readErr)
				}
			}
		}
		if err := container.Terminate(ctx); err != nil {
			t.Logf("%s: cleanup failed: %v", name, err)
		}
	})
}
