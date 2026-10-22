package action

import (
	"context"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/Maksim-Gr/kkon/internal/prompt/prompttest"
	"github.com/Maksim-Gr/kkon/internal/util"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newCmd returns a command wired like kkon's: root persistent --dry-run and
// --output flags, and a local --yes flag, set from args.
func newCmd(t *testing.T, args ...string) *cobra.Command {
	t.Helper()
	root := &cobra.Command{Use: "kkon", SilenceUsage: true, SilenceErrors: true}
	root.PersistentFlags().Bool("dry-run", false, "")
	root.PersistentFlags().StringP("output", "o", "text", "")
	cmd := &cobra.Command{Use: "act", RunE: func(*cobra.Command, []string) error { return nil }}
	cmd.Flags().Bool("yes", false, "")
	root.AddCommand(cmd)
	root.SetArgs(append([]string{"act"}, args...))
	require.NoError(t, root.Execute())
	cmd.SetContext(context.Background())
	return cmd
}

// captureStdout runs fn with os.Stdout redirected and returns what it printed.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	orig := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	fn()

	require.NoError(t, w.Close())
	out, err := io.ReadAll(r)
	require.NoError(t, err)
	return string(out)
}

// recordingSpec returns a pause-like Spec whose Op records targets and fails
// for the connector named "bad".
func recordingSpec(attempted *[]string) Spec {
	return Spec{
		Verb:       "pause",
		SuccessFmt: "Pause requested for %s\n",
		Op: func(_ context.Context, t Target) error {
			*attempted = append(*attempted, t.String())
			if t.Connector == "bad" {
				return errors.New("boom")
			}
			return nil
		},
	}
}

func intPtr(i int) *int { return &i }

func TestRun_SingleJSON(t *testing.T) {
	var attempted []string
	out := captureStdout(t, func() {
		require.NoError(t, Run(newCmd(t, "-o", "json"), recordingSpec(&attempted), Connectors([]string{"alpha"})))
	})
	assert.JSONEq(t, `{"name": "alpha", "action": "pause", "result": "ok"}`, out)
	assert.Equal(t, []string{"alpha"}, attempted)
}

func TestRun_SingleFailureNamesTarget(t *testing.T) {
	var attempted []string
	err := Run(newCmd(t), recordingSpec(&attempted), Connectors([]string{"bad"}))
	require.Error(t, err)
	assert.Equal(t, "failed to pause bad: boom", err.Error())
}

func TestRun_TaskJSON(t *testing.T) {
	spec := Spec{Verb: "restart", ConfirmSingle: true, Op: func(context.Context, Target) error { return nil }}
	out := captureStdout(t, func() {
		require.NoError(t, Run(newCmd(t, "-o", "json", "--yes"), spec, []Target{{Connector: "alpha", Task: intPtr(1)}}))
	})
	assert.JSONEq(t, `{"name": "alpha", "task": 1, "action": "restart", "result": "ok"}`, out)
}

func TestRun_BatchContinuesPastFailure(t *testing.T) {
	var attempted []string
	var err error
	out := captureStdout(t, func() {
		err = Run(newCmd(t, "--yes"), recordingSpec(&attempted), Connectors([]string{"a", "bad", "c"}))
	})
	require.Error(t, err)
	assert.Equal(t, "failed to pause 1 of 3 connector(s)", err.Error())
	assert.Equal(t, []string{"a", "bad", "c"}, attempted, "all targets must still be attempted")
	assert.Contains(t, out, "2 ok, 1 failed")
}

func TestRun_BatchJSON(t *testing.T) {
	var attempted []string
	out := captureStdout(t, func() {
		_ = Run(newCmd(t, "-o", "json", "--yes"), recordingSpec(&attempted), Connectors([]string{"a", "bad"}))
	})
	assert.JSONEq(t, `[{"name": "a", "action": "pause"}, {"name": "bad", "action": "pause", "error": "boom"}]`, out)
}

func TestRun_BatchSpecReportsWithoutAction(t *testing.T) {
	var attempted []string
	spec := recordingSpec(&attempted)
	spec.Verb = "restore"
	spec.Batch = true
	out := captureStdout(t, func() {
		_ = Run(newCmd(t, "-o", "json", "--yes"), spec, Connectors([]string{"bad"}))
	})
	assert.JSONEq(t, `[{"name": "bad", "error": "boom"}]`, out)
}

func TestRun_CanceledContextSkipsRemaining(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cmd := newCmd(t, "--yes")
	cmd.SetContext(ctx)

	var attempted []string
	var err error
	captureStdout(t, func() {
		err = Run(cmd, recordingSpec(&attempted), Connectors([]string{"a", "b"}))
	})
	require.Error(t, err)
	assert.Empty(t, attempted, "canceled context must not invoke the operation")
}

func TestRun_DryRunJSON(t *testing.T) {
	var attempted []string
	spec := recordingSpec(&attempted)

	single := captureStdout(t, func() {
		require.NoError(t, Run(newCmd(t, "--dry-run", "-o", "json"), spec, Connectors([]string{"alpha"})))
	})
	assert.JSONEq(t, `{"name": "alpha", "action": "pause", "result": "dry-run"}`, single)

	multi := captureStdout(t, func() {
		require.NoError(t, Run(newCmd(t, "--dry-run", "-o", "json"), spec, Connectors([]string{"alpha", "beta"})))
	})
	assert.JSONEq(t, `[
		{"name": "alpha", "action": "pause", "result": "dry-run"},
		{"name": "beta", "action": "pause", "result": "dry-run"}
	]`, multi)

	task := captureStdout(t, func() {
		require.NoError(t, Run(newCmd(t, "--dry-run", "-o", "json"), spec, []Target{{Connector: "alpha", Task: intPtr(0)}}))
	})
	assert.JSONEq(t, `{"name": "alpha", "task": 0, "action": "pause", "result": "dry-run"}`, task)

	assert.Empty(t, attempted)
}

func TestRun_ConfirmsMultipleTargets(t *testing.T) {
	var attempted []string
	prompttest.New(t, prompttest.Confirm("Pause 2 connectors (a, b)?", true))
	captureStdout(t, func() {
		require.NoError(t, Run(newCmd(t), recordingSpec(&attempted), Connectors([]string{"a", "b"})))
	})
	assert.Equal(t, []string{"a", "b"}, attempted)
}

func TestRun_DeclinedConfirmCancels(t *testing.T) {
	var attempted []string
	spec := recordingSpec(&attempted)
	spec.ConfirmSingle = true
	prompttest.New(t, prompttest.Confirm("Pause connector a?", false))

	err := Run(newCmd(t), spec, Connectors([]string{"a"}))
	require.ErrorIs(t, err, util.ErrCanceled)
	assert.Empty(t, attempted)
}

func TestRun_JSONModeCannotConfirm(t *testing.T) {
	var attempted []string
	err := Run(newCmd(t, "-o", "json"), recordingSpec(&attempted), Connectors([]string{"a", "b"}))
	require.ErrorIs(t, err, ErrNonInteractiveJSON)
	assert.Empty(t, attempted)
}

func TestRun_ConfirmEachSkipsDeclined(t *testing.T) {
	var attempted []string
	spec := recordingSpec(&attempted)
	spec.ConfirmEach = func(t Target) (bool, error) { return t.Connector != "b", nil }

	captureStdout(t, func() {
		require.NoError(t, Run(newCmd(t), spec, Connectors([]string{"a", "b", "c"})))
	})
	assert.Equal(t, []string{"a", "c"}, attempted)
}

func TestRun_AllDeclinedIsNothingToDo(t *testing.T) {
	var attempted []string
	spec := recordingSpec(&attempted)
	spec.ConfirmEach = func(Target) (bool, error) { return false, nil }

	err := Run(newCmd(t), spec, Connectors([]string{"a"}))
	require.ErrorIs(t, err, util.ErrNothingToDo)
	assert.Empty(t, attempted)
}

func TestRun_YesSkipsConfirmEach(t *testing.T) {
	var attempted []string
	spec := recordingSpec(&attempted)
	spec.ConfirmEach = func(Target) (bool, error) {
		t.Fatal("ConfirmEach must not run with --yes")
		return false, nil
	}

	captureStdout(t, func() {
		require.NoError(t, Run(newCmd(t, "--yes"), spec, Connectors([]string{"a"})))
	})
	assert.Equal(t, []string{"a"}, attempted)
}
