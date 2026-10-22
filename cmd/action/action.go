// Package action runs a mutating command over its targets: dry-run preview,
// confirmation, the operation itself, and reporting in text or JSON.
package action

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/Maksim-Gr/kkon/internal/prompt"
	"github.com/Maksim-Gr/kkon/internal/util"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

// ErrNonInteractiveJSON is returned when --output json is combined with a
// command invocation that would need to prompt.
var ErrNonInteractiveJSON = errors.New("--output json requires non-interactive usage (pass the connector name and --yes)")

// Target is what an action runs on: a connector, or one of its tasks when
// Task is set.
type Target struct {
	Connector string
	Task      *int
}

// Connectors returns a connector Target for each name.
func Connectors(names []string) []Target {
	targets := make([]Target, len(names))
	for i, n := range names {
		targets[i] = Target{Connector: n}
	}
	return targets
}

// String returns the connector name, or "connector task N" for a task.
func (t Target) String() string {
	if t.Task == nil {
		return t.Connector
	}
	return util.FormatTaskRef(t.Connector, *t.Task)
}

// noun is String with "connector " prepended for connector targets, for
// sentences like "Would pause connector x".
func (t Target) noun() string {
	if t.Task == nil {
		return "connector " + t.Connector
	}
	return t.String()
}

// Spec describes a mutating command run through Run.
type Spec struct {
	// Verb names the action in messages and in JSON output, e.g. "pause".
	Verb string
	// SuccessFmt is printed with the target after a single, non-batch run.
	SuccessFmt string
	// ConfirmSingle asks for confirmation even for a single target.
	ConfirmSingle bool
	// ConfirmDefault is the default answer of the confirmation prompt.
	ConfirmDefault bool
	// DryRunNote, if set, returns extra context appended to the dry-run text.
	DryRunNote func() string
	// ConfirmEach, if set, replaces the single confirmation with a question
	// per target; a target answered false is skipped.
	ConfirmEach func(Target) (bool, error)
	// Batch always reports per-target results, even for one target, as JSON
	// items of {"name","error"} without "action" (the restore shape).
	Batch bool
	// Op performs the action on one target.
	Op func(ctx context.Context, t Target) error
}

// Run performs spec on targets. With --dry-run it only previews; otherwise it
// confirms (unless --yes), then runs Op on each target, continuing past
// failures, and reports the outcome. It returns an error if any Op failed.
func Run(cmd *cobra.Command, spec Spec, targets []Target) error {
	jsonMode := util.IsJSONOutput(cmd)

	if util.IsDryRun(cmd) {
		note := ""
		if spec.DryRunNote != nil {
			note = spec.DryRunNote()
		}
		PrintDryRun(jsonMode, spec.Verb, targets, note)
		return nil
	}

	if yes, _ := cmd.Flags().GetBool("yes"); !yes {
		var err error
		if targets, err = confirm(spec, targets, jsonMode); err != nil {
			return err
		}
		if len(targets) == 0 {
			color.Yellow("Nothing to %s\n", spec.Verb)
			return util.ErrNothingToDo
		}
	}

	if len(targets) == 1 && !spec.Batch {
		t := targets[0]
		if err := spec.Op(cmd.Context(), t); err != nil {
			return fmt.Errorf("failed to %s %s: %w", spec.Verb, t, err)
		}
		if jsonMode {
			PrintOK(spec.Verb, t)
			return nil
		}
		color.Green(spec.SuccessFmt, t)
		return nil
	}

	return runBatch(cmd.Context(), jsonMode, spec, targets)
}

// confirm returns the targets the user agreed to act on.
func confirm(spec Spec, targets []Target, jsonMode bool) ([]Target, error) {
	if spec.ConfirmEach != nil {
		var kept []Target
		for _, t := range targets {
			ok, err := spec.ConfirmEach(t)
			if err != nil {
				return nil, err
			}
			if !ok {
				color.Yellow("Skipping %s\n", t)
				continue
			}
			kept = append(kept, t)
		}
		return kept, nil
	}

	if !spec.ConfirmSingle && len(targets) == 1 {
		return targets, nil
	}
	if jsonMode {
		return nil, ErrNonInteractiveJSON
	}

	titleVerb := strings.ToUpper(spec.Verb[:1]) + spec.Verb[1:]
	message := fmt.Sprintf("%s %s?", titleVerb, targets[0].noun())
	if len(targets) > 1 {
		names := make([]string, len(targets))
		for i, t := range targets {
			names[i] = t.String()
		}
		message = fmt.Sprintf("%s %d connectors (%s)?", titleVerb, len(targets), strings.Join(names, ", "))
	}
	ok, err := prompt.Confirm(message, spec.ConfirmDefault)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, util.ErrCanceled
	}
	return targets, nil
}

// outcome is one target's result in a batch report.
type outcome struct {
	Name   string `json:"name"`
	Action string `json:"action,omitempty"`
	Error  string `json:"error,omitempty"`
}

// runBatch applies spec.Op to each target, continuing past individual
// failures; a canceled context fails the remaining targets without running
// them. It prints a per-target ✓/✗ summary (or a JSON array in json mode).
func runBatch(ctx context.Context, jsonMode bool, spec Spec, targets []Target) error {
	outcomes := make([]outcome, 0, len(targets))
	failed := 0
	stop := util.StartSpinner(fmt.Sprintf("Running %s on %d connector(s)...", spec.Verb, len(targets)))
	for _, t := range targets {
		o := outcome{Name: t.String()}
		if !spec.Batch {
			o.Action = spec.Verb
		}
		if err := ctx.Err(); err != nil {
			o.Error = err.Error()
		} else if err := spec.Op(ctx, t); err != nil {
			o.Error = err.Error()
		}
		if o.Error != "" {
			failed++
		}
		outcomes = append(outcomes, o)
	}
	stop()

	if jsonMode {
		b, _ := json.MarshalIndent(outcomes, "", "  ")
		fmt.Println(string(b))
	} else {
		for _, o := range outcomes {
			if o.Error != "" {
				color.Red("  ✗ %s: %s\n", o.Name, o.Error)
			} else {
				color.Green("  ✓ %s\n", o.Name)
			}
		}
		fmt.Printf("%d ok, %d failed\n", len(outcomes)-failed, failed)
	}

	if failed > 0 {
		return fmt.Errorf("failed to %s %d of %d connector(s)", spec.Verb, failed, len(targets))
	}
	return nil
}

// result is the JSON object for a single target's action or dry-run.
type result struct {
	Name   string `json:"name"`
	Task   *int   `json:"task,omitempty"`
	Action string `json:"action"`
	Result string `json:"result"`
}

func newResult(verb string, t Target, res string) result {
	return result{Name: t.Connector, Task: t.Task, Action: verb, Result: res}
}

// PrintOK emits the JSON object reporting that verb succeeded on t.
func PrintOK(verb string, t Target) {
	b, _ := json.Marshal(newResult(verb, t, "ok"))
	fmt.Println(string(b))
}

// PrintDryRun reports what verb would do to targets without doing it. In
// json mode it emits PrintOK's shape with result "dry-run" (an object for one
// target, an indented array for several); in text mode a single line or a
// bulleted list, with note appended.
func PrintDryRun(jsonMode bool, verb string, targets []Target, note string) {
	if jsonMode {
		if len(targets) == 1 {
			b, _ := json.Marshal(newResult(verb, targets[0], "dry-run"))
			fmt.Println(string(b))
			return
		}
		results := make([]result, len(targets))
		for i, t := range targets {
			results[i] = newResult(verb, t, "dry-run")
		}
		b, _ := json.MarshalIndent(results, "", "  ")
		fmt.Println(string(b))
		return
	}

	if len(targets) == 1 {
		color.Yellow("[dry-run] Would %s %s%s\n", verb, targets[0].noun(), note)
		return
	}
	color.Yellow("[dry-run] Would %s %d connector(s)%s:\n", verb, len(targets), note)
	for _, t := range targets {
		color.Yellow("  - %s\n", t)
	}
}
