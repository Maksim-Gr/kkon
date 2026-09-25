package connector

import (
	"fmt"

	"github.com/Maksim-Gr/kkon/internal/util"

	"github.com/AlecAivazis/survey/v2"
)

func promptSchemaRegistryFormat(cfg map[string]string) error {
	const none = "None (raw JSON, current default)"
	options := []string{none, "Avro", "Protobuf", "JSON Schema"}
	converterClass := map[string]string{
		"Avro":        "io.confluent.connect.avro.AvroConverter",
		"Protobuf":    "io.confluent.connect.protobuf.ProtobufConverter",
		"JSON Schema": "io.confluent.connect.json.JsonSchemaConverter",
	}

	var choice string
	if err := survey.AskOne(&survey.Select{
		Message: "Value format (Confluent Schema Registry converters)?",
		Options: options,
		Default: none,
	}, &choice); err != nil {
		if util.IsSurveyInterrupt(err) {
			return util.ErrCanceled
		}
		return fmt.Errorf("prompt failed: %w", err)
	}
	if choice == none {
		return nil
	}

	defaultURL := ""
	if loaded, err := util.ResolveConfig(); err == nil {
		defaultURL = loaded.SchemaRegistry.URL
	}

	var srURL string
	if err := survey.AskOne(&survey.Input{
		Message: "Schema Registry URL:",
		Default: defaultURL,
	}, &srURL, survey.WithValidator(survey.Required)); err != nil {
		if util.IsSurveyInterrupt(err) {
			return util.ErrCanceled
		}
		return fmt.Errorf("prompt failed: %w", err)
	}

	cfg["value.converter"] = converterClass[choice]
	cfg["value.converter.schema.registry.url"] = srURL

	var sameKey bool
	if err := survey.AskOne(&survey.Confirm{
		Message: "Use the same format for the key?",
		Default: false,
	}, &sameKey); err != nil && util.IsSurveyInterrupt(err) {
		return util.ErrCanceled
	}
	if sameKey {
		cfg["key.converter"] = converterClass[choice]
		cfg["key.converter.schema.registry.url"] = srURL
	}
	return nil
}
