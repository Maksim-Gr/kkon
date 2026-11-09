package tests

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Maksim-Gr/kkon/internal/schemaregistry"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestSchemaRegistryLifecycle(t *testing.T) {
	kafka := KafkaConnectFixture(t)
	ctx := context.Background()
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "confluentinc/cp-schema-registry:" + confluentVersion,
			ExposedPorts: []string{"8081/tcp"},
			Networks:     []string{kafka.Network},
			Env: map[string]string{
				"SCHEMA_REGISTRY_HOST_NAME":                    "schema-registry",
				"SCHEMA_REGISTRY_KAFKASTORE_BOOTSTRAP_SERVERS": "kafka:9092",
				"SCHEMA_REGISTRY_LISTENERS":                    "http://0.0.0.0:8081",
			},
			WaitingFor: wait.ForHTTP("/subjects").WithPort("8081/tcp").WithStartupTimeout(120 * time.Second),
		},
		Started: true,
	})
	cleanupKafkaContainer(t, "schema-registry", container)
	require.NoError(t, err)
	host, err := container.Host(ctx)
	require.NoError(t, err)
	port, err := container.MappedPort(ctx, "8081")
	require.NoError(t, err)
	client := schemaregistry.NewClient(fmt.Sprintf("http://%s:%s", host, port.Port()), "", "")
	for _, test := range []struct {
		name, schemaType, dependency, schema, referenceName string
	}{
		{"avro", "AVRO", `{"type":"record","name":"Common","fields":[{"name":"id","type":"string"}]}`, `{"type":"record","name":"Event","fields":[{"name":"common","type":"Common"}]}`, "Common"},
		{"json", "JSON", `{"type":"object","properties":{"id":{"type":"string"}}}`, `{"type":"object","properties":{"common":{"$ref":"common.json"}}}`, "common.json"},
		{"protobuf", "PROTOBUF", `syntax = "proto3"; message Common { string id = 1; }`, `syntax = "proto3"; import "common.proto"; message Event { Common common = 1; }`, "common.proto"},
	} {
		t.Run(test.name, func(t *testing.T) {
			dependencySubject := "common-" + test.name
			_, err := client.Publish(ctx, dependencySubject, schemaregistry.Schema{Schema: test.dependency, SchemaType: test.schemaType, References: []schemaregistry.Reference{}})
			require.NoError(t, err)
			input := schemaregistry.Schema{
				Schema: test.schema, SchemaType: test.schemaType,
				References: []schemaregistry.Reference{{Name: test.referenceName, Subject: dependencySubject, Version: 1}},
			}
			subject := "event-" + test.name
			_, err = client.Check(ctx, subject, input)
			require.ErrorContains(t, err, "compatibility cannot be evaluated")
			registration, err := client.Publish(ctx, subject, input)
			require.NoError(t, err)
			got, err := client.Get(ctx, subject, "latest")
			require.NoError(t, err)
			assert.Equal(t, registration.ID, got.ID)
			assert.Equal(t, input.SchemaType, got.SchemaType)
			assert.Equal(t, input.References, got.References)
			check, err := client.Check(ctx, subject, input)
			require.NoError(t, err)
			assert.True(t, check.IsCompatible)
			versions, err := client.Versions(ctx, subject)
			require.NoError(t, err)
			assert.Equal(t, []int{1}, versions)
		})
	}
	// A real incompatible evolution must be rejected by both check and publish.
	incompatible := schemaregistry.Schema{SchemaType: "AVRO", Schema: `{"type":"record","name":"Common","fields":[{"name":"id","type":"int"}]}`}
	check, err := client.Check(ctx, "common-avro", incompatible)
	require.NoError(t, err)
	assert.False(t, check.IsCompatible)
	assert.NotEmpty(t, check.Messages)
	_, err = client.Publish(ctx, "common-avro", incompatible)
	var apiErr *schemaregistry.APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, 409, apiErr.Status)
	_, err = client.Publish(ctx, "invalid", schemaregistry.Schema{SchemaType: "AVRO", Schema: "invalid schema"})
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, 422, apiErr.Status)

	t.Run("transitive policy", func(t *testing.T) {
		const subject = "history"
		for _, schema := range []string{
			`{"type":"record","name":"History","fields":[{"name":"id","type":"string"}]}`,
			`{"type":"record","name":"History","fields":[]}`,
		} {
			_, err := client.Publish(ctx, subject, schemaregistry.Schema{SchemaType: "AVRO", Schema: schema})
			require.NoError(t, err)
		}
		candidate := schemaregistry.Schema{SchemaType: "AVRO", Schema: `{"type":"record","name":"History","fields":[{"name":"id","type":"int","default":0}]}`}
		check, err := client.Check(ctx, subject, candidate)
		require.NoError(t, err)
		require.True(t, check.IsCompatible, "candidate is backward compatible with latest version")
		request, err := http.NewRequestWithContext(ctx, http.MethodPut, fmt.Sprintf("http://%s:%s/config/%s", host, port.Port(), subject), strings.NewReader(`{"compatibility":"BACKWARD_TRANSITIVE"}`))
		require.NoError(t, err)
		request.Header.Set("Content-Type", "application/vnd.schemaregistry.v1+json")
		response, err := (&http.Client{Timeout: 30 * time.Second}).Do(request)
		require.NoError(t, err)
		_ = response.Body.Close()
		require.Equal(t, http.StatusOK, response.StatusCode)
		check, err = client.Check(ctx, subject, candidate)
		require.NoError(t, err)
		assert.False(t, check.IsCompatible, "must compare against the original version under transitive policy")
		_, err = client.Publish(ctx, subject, candidate)
		require.ErrorAs(t, err, &apiErr)
		assert.Equal(t, 409, apiErr.Status)
	})
}
