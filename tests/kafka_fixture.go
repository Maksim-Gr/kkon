package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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
}

// KafkaConnectFixture starts a full Kafka + Kafka Connect stack and returns a test fixture.
func KafkaConnectFixture(t *testing.T) *KafkaConnectTestFixture {
	t.Helper()
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
		WaitingFor: wait.ForListeningPort("2181/tcp").WithStartupTimeout(30 * time.Second),
		Name:       "zookeeper",
	}
	zooC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: zooReq,
		Started:          true,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = zooC.Terminate(ctx) })

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
		WaitingFor: wait.ForListeningPort("9092/tcp").WithStartupTimeout(90 * time.Second),
		Name:       "kafka",
	}
	kafkaC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: kafkaReq,
		Started:          true,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = kafkaC.Terminate(ctx) })

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
		WaitingFor: wait.ForHTTP("/").WithPort("8083/tcp").WithStartupTimeout(120 * time.Second),
		Name:       "kafkaconnect",
	}
	connectC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: connectReq,
		Started:          true,
	})

	require.NoError(t, err)
	t.Cleanup(func() { _ = connectC.Terminate(ctx) })

	connectHost, err := connectC.Host(ctx)
	require.NoError(t, err)
	connectPort, err := connectC.MappedPort(ctx, "8083")
	require.NoError(t, err)

	connectURL := fmt.Sprintf("http://%s:%s", connectHost, connectPort.Port())
	WaitForKafkaConnectStartUp(t, connectURL, 20*time.Second)

	return &KafkaConnectTestFixture{ // Return pointer
		Container: connectC,
		URL:       connectURL,
	}
}
