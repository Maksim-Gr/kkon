package connector

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Maksim-Gr/kkon/cmd/action"
	"github.com/Maksim-Gr/kkon/internal/connector"
	"github.com/Maksim-Gr/kkon/internal/prompt"
	"github.com/Maksim-Gr/kkon/internal/util"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

// configMapFromFile extracts a connector config map from a connector JSON file,
// accepting either the wrapped {"name":..,"config":{..}} form or a flat config
// map. For the wrapped form the top-level name is folded into the config.
// Numbers, booleans and nulls become the strings Kafka Connect stores; nested
// values, other top-level fields and a missing name are errors.
func configMapFromFile(b []byte) (map[string]string, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(b, &top); err != nil {
		return nil, errors.New("is not a JSON object")
	}

	var name string
	rawConfig := top
	if raw, wrapped := top["config"]; wrapped {
		for k := range top {
			if k != "name" && k != "config" {
				return nil, fmt.Errorf("has unsupported top-level field %q (only \"name\" and \"config\" are allowed)", k)
			}
		}
		rawConfig = nil
		if err := json.Unmarshal(raw, &rawConfig); err != nil || rawConfig == nil {
			return nil, errors.New(`has a "config" that is not a JSON object`)
		}
		if n, ok := top["name"]; ok {
			if err := json.Unmarshal(n, &name); err != nil {
				return nil, errors.New(`has a "name" that is not a string`)
			}
		}
	}

	cfg := make(map[string]string, len(rawConfig)+1)
	for k, raw := range rawConfig {
		d := json.NewDecoder(bytes.NewReader(raw))
		d.UseNumber()
		var v any
		if err := d.Decode(&v); err != nil {
			return nil, fmt.Errorf("has an invalid value for %q", k)
		}
		switch v.(type) {
		case map[string]any, []any:
			return nil, fmt.Errorf("has a nested value for %q (config values must be strings, numbers or booleans)", k)
		}
		cfg[k] = anyToString(v)
	}

	switch {
	case name != "" && cfg["name"] == "":
		cfg["name"] = name
	case name != "" && cfg["name"] != name:
		return nil, fmt.Errorf("has name %q but config name %q", name, cfg["name"])
	}
	if cfg["name"] == "" {
		return nil, errors.New(`has no "name"`)
	}
	return cfg, nil
}

// argOrEmpty returns the first positional arg, or "" if none was provided.
func argOrEmpty(args []string) string {
	if len(args) > 0 {
		return args[0]
	}
	return ""
}

// lifecycleRunE returns a RunE shared by the lifecycle commands (pause,
// resume, restart, delete). Targets come from positional args, --all, or an
// interactive multi-select; action.Run does the rest, with op applied to each.
func lifecycleRunE(spec action.Spec, op func(ctx context.Context, client *connector.Client, name string) error) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		all, _ := cmd.Flags().GetBool("all")
		if util.IsJSONOutput(cmd) && len(args) == 0 && !all {
			return action.ErrNonInteractiveJSON
		}

		client, err := util.NewKafkaConnectClient()
		if err != nil {
			return err
		}

		names, err := util.ResolveConnectorNames(cmd.Context(), client, args, all)
		if err != nil {
			return err
		}

		spec.Op = func(ctx context.Context, t action.Target) error {
			return op(ctx, client, t.Connector)
		}
		return action.Run(cmd, spec, action.Connectors(names))
	}
}

// checkConfig runs server-side validation on a connector config and returns
// the server's error count with one "field: message" line per problem. It
// reports no problems when connector.class is unset, and returns err only when
// validation itself could not run; both cases let submission proceed.
func checkConfig(ctx context.Context, client *connector.Client, cfg map[string]string) (errorCount int, problems []string, err error) {
	class := cfg["connector.class"]
	if class == "" {
		return 0, nil, nil
	}

	res, err := client.ValidateConnectorConfig(ctx, class, cfg)
	if err != nil {
		return 0, nil, err
	}
	for _, c := range res.Configs {
		for _, e := range c.Value.Errors {
			problems = append(problems, c.Value.Name+": "+e)
		}
	}
	return res.ErrorCount, problems, nil
}

// validateConfigOrConfirm reports checkConfig's problems and asks whether to
// submit anyway. It returns true if the caller should proceed.
func validateConfigOrConfirm(ctx context.Context, client *connector.Client, cfg map[string]string) bool {
	errorCount, problems, err := checkConfig(ctx, client, cfg)
	if err != nil {
		color.Yellow("Could not validate config: %v\n", err)
		return true
	}
	if errorCount == 0 {
		return true
	}

	color.Red("\nValidation found %d error(s):\n", errorCount)
	for _, p := range problems {
		color.Red("  %s\n", p)
	}

	proceed, err := prompt.Confirm("Submit anyway?", false)
	return err == nil && proceed
}

// validateConfigStrict returns checkConfig's problems as an error. It is the
// non-interactive counterpart of validateConfigOrConfirm for --output json.
func validateConfigStrict(ctx context.Context, client *connector.Client, cfg map[string]string) error {
	errorCount, problems, err := checkConfig(ctx, client, cfg)
	if err != nil || errorCount == 0 {
		return nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "validation found %d error(s):", errorCount)
	for _, p := range problems {
		fmt.Fprintf(&b, "\n  %s", p)
	}
	return errors.New(b.String())
}

// editConnectorConfig fetches the named connector's live config, lets the user
// edit fields interactively, shows a diff, validates, and applies the change.
// Applying restarts the connector, so it then verifies the connector comes back
// RUNNING and offers to revert to the original config if it does not.
// User cancels return util.ErrCanceled; with dryRun the diff is shown but the
// change is not applied.
func editConnectorConfig(ctx context.Context, client *connector.Client, selected string, dryRun bool) error {
	connectorConfig, err := client.GetConnectorConfig(ctx, selected)
	if err != nil {
		return fmt.Errorf("failed to get connector config: %w", err)
	}

	// Snapshot the original config for diff display and potential revert.
	original := make(map[string]string, len(connectorConfig))
	for k, v := range connectorConfig {
		original[k] = v
	}

	for {
		pretty, err := util.ToPrettyJSON(connectorConfig)
		if err != nil {
			return fmt.Errorf("failed to format config: %w", err)
		}
		color.Cyan("\n Current config for %s:\n", selected)
		fmt.Println(pretty)

		fields := make([]string, 0, len(connectorConfig))
		for k := range connectorConfig {
			fields = append(fields, k)
		}
		sort.Strings(fields)

		i, err := prompt.Select("Which field do you want to change?", fields, prompt.Options{})
		if err != nil {
			return err
		}
		fieldToChange := fields[i]

		newValue, err := prompt.Input(fmt.Sprintf("New value for %s (current: %v):", fieldToChange, connectorConfig[fieldToChange]), prompt.Options{})
		if err != nil {
			return err
		}
		connectorConfig[fieldToChange] = newValue

		more, err := prompt.Confirm("Change another field?", false)
		if err != nil {
			return err
		}
		if !more {
			break
		}
	}

	// Compute and display changed fields.
	var changedKeys []string
	for k, newV := range connectorConfig {
		if oldV, exists := original[k]; exists && oldV != newV {
			changedKeys = append(changedKeys, k)
		}
	}

	if len(changedKeys) == 0 {
		color.Yellow("No changes made\n")
		return nil
	}

	sort.Strings(changedKeys)
	maxKeyLen := 0
	for _, k := range changedKeys {
		if len(k) > maxKeyLen {
			maxKeyLen = len(k)
		}
	}
	color.Cyan("\nChanges:")
	for _, k := range changedKeys {
		fmt.Printf("  %-*s  %s  →  %s\n", maxKeyLen, k, original[k], connectorConfig[k])
	}

	if dryRun {
		color.Yellow("[dry-run] Would apply %d change(s) to %s\n", len(changedKeys), selected)
		return nil
	}

	confirm, err := prompt.Confirm("Apply this config to "+selected+"?", true)
	if err != nil {
		return err
	}
	if !confirm {
		return util.ErrCanceled
	}

	if !validateConfigOrConfirm(ctx, client, connectorConfig) {
		return util.ErrCanceled
	}

	if err := client.UpdateConnectorConfig(ctx, selected, connectorConfig); err != nil {
		return fmt.Errorf("failed to update connector: %w", err)
	}
	color.Green("Connector %s updated; restarting with new config...\n", selected)

	verifyConnectorOrRevert(ctx, client, selected, original)
	return nil
}

// verifyPollDelay is the wait between status polls after a config change.
var verifyPollDelay = 2 * time.Second

// verifyConnectorOrRevert polls the connector's status after a config change.
// Updating a config restarts the connector and its tasks, so this waits for it
// to settle and reports whether it is RUNNING. If it is not healthy, it offers
// to revert to the previous config.
func verifyConnectorOrRevert(ctx context.Context, client *connector.Client, name string, original map[string]string) {
	status, healthy := waitForConnectorRunning(ctx, client, name, 5, verifyPollDelay)

	if healthy {
		color.Green("Connector %s is up and running: %s\n", name, util.ColorState(status.Connector.State))
		return
	}

	if status.Name == "" {
		color.Yellow("Could not verify connector status\n")
		return
	}

	color.Red("Connector %s did not come back RUNNING.\n", name)
	color.Red("  connector: %s\n", util.ColorState(status.Connector.State))
	for _, t := range status.Tasks {
		fmt.Printf("  task %d: %s\n", t.ID, util.ColorState(t.State))
	}

	revert, err := prompt.Confirm(fmt.Sprintf("Connector %s is not RUNNING. Revert to previous config?", name), true)
	if err != nil || !revert {
		color.Yellow("Keeping new config in place\n")
		return
	}

	if err := client.UpdateConnectorConfig(ctx, name, original); err != nil {
		color.Red("Failed to revert connector: %v\n", err)
		return
	}
	color.Green("Reverted %s to previous config\n", name)
}

// waitForConnectorRunning polls the connector's status up to attempts times,
// sleeping delay between tries, and returns once it is healthy or attempts are
// exhausted or ctx is canceled. The returned bool reports whether it became
// healthy; the returned status is the last one observed (zero-valued, with
// empty Name, if every poll errored).
func waitForConnectorRunning(ctx context.Context, client *connector.Client, name string, attempts int, delay time.Duration) (connector.Status, bool) {
	var status connector.Status
	for i := 0; i < attempts; i++ {
		select {
		case <-ctx.Done():
			return status, false
		case <-time.After(delay):
		}
		s, err := client.GetConnectorStatus(ctx, name)
		if err != nil {
			continue
		}
		status = s
		if connectorHealthy(status) {
			return status, true
		}
	}
	return status, false
}

// printConnectorStatus prints the state of a connector and its tasks.
// For FAILED tasks it shows the first 3 lines of the error trace.
func printConnectorStatus(name string, status connector.Status) {
	fmt.Printf("  %s  %s\n", name, util.ColorState(status.Connector.State))
	for _, t := range status.Tasks {
		fmt.Printf("    Task %d: %s\n", t.ID, util.ColorState(t.State))
		if t.State == "FAILED" && t.Trace != "" {
			lines := strings.Split(strings.TrimRight(t.Trace, "\n"), "\n")
			shown := lines
			if len(lines) > 3 {
				shown = lines[:3]
			}
			for _, line := range shown {
				color.Yellow("      %s\n", line)
			}
			if len(lines) > 3 {
				color.Yellow("      ...\n")
				fmt.Printf("      To see full trace run: kkon task get --connector %s --id %d\n", name, t.ID)
			}
		}
	}
}

// connectorHealthy reports whether the connector and all its tasks are RUNNING.
func connectorHealthy(status connector.Status) bool {
	if status.Connector.State != "RUNNING" {
		return false
	}
	for _, t := range status.Tasks {
		if t.State != "RUNNING" {
			return false
		}
	}
	return true
}
