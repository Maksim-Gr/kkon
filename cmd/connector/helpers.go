package connector

import (
	"context"
	"encoding/json"

	"github.com/Maksim-Gr/kkon/internal/connector"

	"github.com/AlecAivazis/survey/v2"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

// configMapFromFile extracts a connector config map from a connector JSON file,
// accepting either the wrapped {"name":..,"config":{..}} form or a flat config
// map. It returns nil if neither form parses, in which case validation is skipped.
func configMapFromFile(b []byte) map[string]string {
	var wrapper struct {
		Config map[string]string `json:"config"`
	}
	if err := json.Unmarshal(b, &wrapper); err == nil && len(wrapper.Config) > 0 {
		return wrapper.Config
	}
	var flat map[string]string
	if err := json.Unmarshal(b, &flat); err == nil {
		return flat
	}
	return nil
}

// argOrEmpty returns the first positional arg, or "" if none was provided.
func argOrEmpty(args []string) string {
	if len(args) > 0 {
		return args[0]
	}
	return ""
}

// isDryRun reports whether the global --dry-run flag is set.
func isDryRun(cmd *cobra.Command) bool {
	return cmd.Root().PersistentFlags().Lookup("dry-run").Value.String() == "true"
}

// validateConfigOrConfirm runs server-side validation on a connector config.
// It returns true if the caller should proceed with submission. When the
// connector.class is unknown or validation can't run, it proceeds without
// blocking. When validation reports errors, it prints them and asks the user
// whether to submit anyway.
func validateConfigOrConfirm(ctx context.Context, client *connector.Client, cfg map[string]string) bool {
	class := cfg["connector.class"]
	if class == "" {
		return true
	}

	res, err := client.ValidateConnectorConfig(ctx, class, cfg)
	if err != nil {
		color.Yellow("Could not validate config: %v\n", err)
		return true
	}
	if res.ErrorCount == 0 {
		return true
	}

	color.Red("\nValidation found %d error(s):\n", res.ErrorCount)
	for _, c := range res.Configs {
		for _, e := range c.Value.Errors {
			color.Red("  %s: %s\n", c.Value.Name, e)
		}
	}

	var proceed bool
	if err := survey.AskOne(&survey.Confirm{
		Message: "Submit anyway?",
		Default: false,
	}, &proceed); err != nil {
		return false
	}
	return proceed
}
