package connector

import (
	"github.com/Maksim-Gr/kkon/internal/prompt"
	"github.com/Maksim-Gr/kkon/internal/util"
)

func promptSchemaRegistryFormat(cfg map[string]string) error {
	const none = "None (raw JSON, current default)"
	options := []string{none, "Avro", "Protobuf", "JSON Schema"}
	converterClass := map[string]string{
		"Avro":        "io.confluent.connect.avro.AvroConverter",
		"Protobuf":    "io.confluent.connect.protobuf.ProtobufConverter",
		"JSON Schema": "io.confluent.connect.json.JsonSchemaConverter",
	}

	i, err := prompt.Select("Value format (Confluent Schema Registry converters)?", options, prompt.Options{Default: none})
	if err != nil {
		return err
	}
	choice := options[i]
	if choice == none {
		return nil
	}

	defaultURL := ""
	if loaded, err := util.ResolveConfig(); err == nil {
		defaultURL = loaded.SchemaRegistry.URL
	}

	srURL, err := prompt.Input("Schema Registry URL:", prompt.Options{Default: defaultURL, Validate: prompt.Required})
	if err != nil {
		return err
	}

	cfg["value.converter"] = converterClass[choice]
	cfg["value.converter.schema.registry.url"] = srURL

	sameKey, err := prompt.Confirm("Use the same format for the key?", false)
	if err != nil {
		return err
	}
	if sameKey {
		cfg["key.converter"] = converterClass[choice]
		cfg["key.converter.schema.registry.url"] = srURL
	}
	return nil
}
