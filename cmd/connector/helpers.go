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
			confirm, err := prompt.Confirm(message, spec.confirmDefault)
			if err != nil {
				return err
			}
			if !confirm {
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
