package connector

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"sort"
	"strings"
	"time"

	"github.com/Maksim-Gr/kkon/internal/connector"
	"github.com/Maksim-Gr/kkon/internal/util"

	"github.com/AlecAivazis/survey/v2"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

// configMapFromFile extracts a connector config map from a connector JSON file,
// accepting either the wrapped {"name":..,"config":{..}} form or a flat config
// map. For the wrapped form the top-level name is folded into the config, since
// the validate endpoint requires it. wrapped reports which form was found. It
// returns a nil map if neither form parses, in which case validation is skipped.
func configMapFromFile(b []byte) (cfg map[string]string, wrapped bool) {
	var wrapper struct {
		Name   string            `json:"name"`
		Config map[string]string `json:"config"`
	}
	if err := json.Unmarshal(b, &wrapper); err == nil && len(wrapper.Config) > 0 {
		if wrapper.Name != "" && wrapper.Config["name"] == "" {
			cfg := make(map[string]string, len(wrapper.Config)+1)
			maps.Copy(cfg, wrapper.Config)
			cfg["name"] = wrapper.Name
			return cfg, true
		}
		return wrapper.Config, true
	}
	var flat map[string]string
	if err := json.Unmarshal(b, &flat); err == nil {
		return flat, false
	}
	return nil, false
}

// argOrEmpty returns the first positional arg, or "" if none was provided.
func argOrEmpty(args []string) string {
	if len(args) > 0 {
		return args[0]
	}
	return ""
}

// errNonInteractiveJSON is returned when --output json is combined with a
// command invocation that would need to prompt.
var errNonInteractiveJSON = errors.New("--output json requires non-interactive usage (pass the connector name and --yes)")

// printActionJSON emits a single action result object to stdout for --output json mode.
func printActionJSON(name, action string) {
	b, _ := json.Marshal(map[string]string{"name": name, "action": action, "result": "ok"})
	fmt.Println(string(b))
}

// printDryRunPreview reports what a lifecycle command would do to names,
// without doing it. In json mode it emits the same {"name","action","result"}
// shape as printActionJSON with result "dry-run" (a single object for one
// name, an indented array for multiple), so scripts get consistent output
// whether an action ran or was only previewed. In text mode it prints the
// existing single-line or bulleted preview, with note appended for extra
// context (e.g. restart's includeTasks/onlyFailed flags).
func printDryRunPreview(jsonMode bool, verb string, names []string, note string) {
	if jsonMode {
		type dryRunResult struct {
			Name   string `json:"name"`
			Action string `json:"action"`
			Result string `json:"result"`
		}
		results := make([]dryRunResult, len(names))
		for i, n := range names {
			results[i] = dryRunResult{Name: n, Action: verb, Result: "dry-run"}
		}
		if len(results) == 1 {
			b, _ := json.Marshal(results[0])
			fmt.Println(string(b))
			return
		}
		b, _ := json.MarshalIndent(results, "", "  ")
		fmt.Println(string(b))
		return
	}

	if len(names) == 1 {
		color.Yellow("[dry-run] Would %s connector %s%s\n", verb, names[0], note)
		return
	}
	color.Yellow("[dry-run] Would %s %d connector(s)%s:\n", verb, len(names), note)
	for _, n := range names {
		color.Yellow("  - %s\n", n)
	}
}

// printResultLines reports the outcome of a batch operation: one ✓/✗ line per
// (name, err) pair plus a "N ok, M failed" summary in text mode, or the given
// pre-marshaled JSON in json mode. names and errs must be parallel slices
// (errs[i] == "" means names[i] succeeded). Returns the number of failures.
func printResultLines(jsonMode bool, jsonBytes []byte, names []string, errs []string) int {
	failed := 0
	for _, e := range errs {
		if e != "" {
			failed++
		}
	}

	if jsonMode {
		fmt.Println(string(jsonBytes))
		return failed
	}

	for i, name := range names {
		if errs[i] != "" {
			color.Red("  ✗ %s: %s\n", name, errs[i])
		} else {
			color.Green("  ✓ %s\n", name)
		}
	}
	fmt.Printf("%d ok, %d failed\n", len(names)-failed, failed)
	return failed
}

// lifecycleSpec describes a connector lifecycle command (pause, resume,
// restart, delete) run through lifecycleRunE.
type lifecycleSpec struct {
	verb           string // e.g. "pause"; also used in messages and json output
	successFmt     string // printed with the connector name for a single target
	confirmSingle  bool   // ask for confirmation even for a single connector
	confirmDefault bool   // default answer of the confirmation prompt
	dryRunNote     func() string
	op             func(ctx context.Context, client *connector.Client, name string) error
}

// lifecycleRunE returns a RunE shared by the lifecycle commands. Targets come
// from positional args, --all, or an interactive multi-select; a single target
// keeps the original one-connector behavior, while multiple targets are
// processed via runBulk with a per-connector summary.
func lifecycleRunE(spec lifecycleSpec) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		jsonMode := util.IsJSONOutput(cmd)
		all, _ := cmd.Flags().GetBool("all")
		yes, _ := cmd.Flags().GetBool("yes")

		if jsonMode && len(args) == 0 && !all {
			return errNonInteractiveJSON
		}

		client, err := util.NewKafkaConnectClient()
		if err != nil {
			return err
		}

		names, err := util.ResolveConnectorNames(cmd.Context(), client, args, all)
		if err != nil {
			return err
		}

		if util.IsDryRun(cmd) {
			note := ""
			if spec.dryRunNote != nil {
				note = spec.dryRunNote()
			}
			printDryRunPreview(jsonMode, spec.verb, names, note)
			return nil
		}

		if !yes && (spec.confirmSingle || len(names) > 1) {
			if jsonMode {
				return errNonInteractiveJSON
			}
			titleVerb := strings.ToUpper(spec.verb[:1]) + spec.verb[1:]
			message := fmt.Sprintf("%s connector %s?", titleVerb, names[0])
			if len(names) > 1 {
				message = fmt.Sprintf("%s %d connectors (%s)?", titleVerb, len(names), strings.Join(names, ", "))
			}
			var confirm bool
			if err := survey.AskOne(&survey.Confirm{
				Message: message,
				Default: spec.confirmDefault,
			}, &confirm); err != nil || !confirm {
				return util.ErrCanceled
			}
		}

		if len(names) == 1 {
			name := names[0]
			if err := spec.op(cmd.Context(), client, name); err != nil {
				return fmt.Errorf("failed to %s %s: %w", spec.verb, name, err)
			}
			if jsonMode {
				printActionJSON(name, spec.verb)
				return nil
			}
			color.Green(spec.successFmt, name)
			return nil
		}

		return runBulk(cmd.Context(), jsonMode, spec.verb, names, func(ctx context.Context, name string) error {
			return spec.op(ctx, client, name)
		})
	}
}

// runBulk applies op to each name, continuing past individual failures. It
// prints a per-connector ✓/✗ summary (or a JSON results array in json mode)
// and returns an error when any operation failed.
func runBulk(ctx context.Context, jsonMode bool, verb string, names []string, op func(ctx context.Context, name string) error) error {
	type bulkResult struct {
		Name   string `json:"name"`
		Action string `json:"action"`
		Error  string `json:"error,omitempty"`
	}

	results := make([]bulkResult, 0, len(names))
	errs := make([]string, 0, len(names))
	stop := util.StartSpinner(fmt.Sprintf("Running %s on %d connector(s)...", verb, len(names)))
	for _, name := range names {
		r := bulkResult{Name: name, Action: verb}
		if err := ctx.Err(); err != nil {
			r.Error = err.Error()
		} else if err := op(ctx, name); err != nil {
			r.Error = err.Error()
		}
		results = append(results, r)
		errs = append(errs, r.Error)
	}
	stop()

	b, _ := json.MarshalIndent(results, "", "  ")
	failed := printResultLines(jsonMode, b, names, errs)

	if failed > 0 {
		return fmt.Errorf("failed to %s %d of %d connector(s)", verb, failed, len(names))
	}
	return nil
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

// validateConfigStrict runs server-side validation and returns an error listing
// the failing fields. It is the non-interactive counterpart of
// validateConfigOrConfirm for contexts where prompting is not possible
// (--output json). Like the interactive variant, it proceeds when the
// connector.class is unknown or validation itself can't run.
func validateConfigStrict(ctx context.Context, client *connector.Client, cfg map[string]string) error {
	class := cfg["connector.class"]
	if class == "" {
		return nil
	}

	res, err := client.ValidateConnectorConfig(ctx, class, cfg)
	if err != nil || res.ErrorCount == 0 {
		return nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "validation found %d error(s):", res.ErrorCount)
	for _, c := range res.Configs {
		for _, e := range c.Value.Errors {
			fmt.Fprintf(&b, "\n  %s: %s", c.Value.Name, e)
		}
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
	connectorConfig, err := client.GetConnectorConfigJSON(ctx, selected)
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

		var fieldToChange string
		if err := survey.AskOne(&survey.Select{
			Message: "Which field do you want to change?",
			Options: fields,
		}, &fieldToChange); err != nil {
			return util.ErrCanceled
		}

		var newValue string
		if err := survey.AskOne(&survey.Input{
			Message: fmt.Sprintf("New value for %s (current: %v):", fieldToChange, connectorConfig[fieldToChange]),
		}, &newValue); err != nil {
			return util.ErrCanceled
		}
		connectorConfig[fieldToChange] = newValue

		var more bool
		if err := survey.AskOne(&survey.Confirm{
			Message: "Change another field?",
			Default: false,
		}, &more); err != nil {
			return util.ErrCanceled
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

	var confirm bool
	if err := survey.AskOne(&survey.Confirm{
		Message: "Apply this config to " + selected + "?",
		Default: true,
	}, &confirm); err != nil || !confirm {
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

// verifyConnectorOrRevert polls the connector's status after a config change.
// Updating a config restarts the connector and its tasks, so this waits for it
// to settle and reports whether it is RUNNING. If it is not healthy, it offers
// to revert to the previous config.
func verifyConnectorOrRevert(ctx context.Context, client *connector.Client, name string, original map[string]string) {
	status, healthy := waitForConnectorRunning(ctx, client, name, 5, 2*time.Second)

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

	var revert bool
	if err := survey.AskOne(&survey.Confirm{
		Message: fmt.Sprintf("Connector %s is not RUNNING. Revert to previous config?", name),
		Default: true,
	}, &revert); err != nil || !revert {
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
