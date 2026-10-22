// Package prompttest provides a scripted prompt.Prompter for tests.
package prompttest

import (
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/Maksim-Gr/kkon/internal/prompt"
)

// Step is one expected prompt and its answer. Build steps with Select,
// MultiSelect, Input, Secret, Confirm and Cancel.
type Step struct {
	kind     string
	contains string
	answer   any
}

// Select answers a Select whose message contains msg with the given option.
func Select(msg, option string) Step { return Step{"select", msg, option} }

// MultiSelect answers a MultiSelect whose message contains msg with options.
func MultiSelect(msg string, options ...string) Step {
	return Step{"multiselect", msg, options}
}

// Input answers an Input whose message contains msg. An empty answer takes
// the prompt's default.
func Input(msg, answer string) Step { return Step{"input", msg, answer} }

// Secret answers a Secret whose message contains msg.
func Secret(msg, answer string) Step { return Step{"secret", msg, answer} }

// Confirm answers a Confirm whose message contains msg.
func Confirm(msg string, answer bool) Step { return Step{"confirm", msg, answer} }

// Cancel interrupts the next prompt, of any kind, whose message contains msg.
func Cancel(msg string) Step { return Step{"cancel", msg, nil} }

// Script is a Prompter that expects its steps in order. Any mismatch fails the
// test, as does a step still unused when the test ends.
type Script struct {
	t     testing.TB
	mu    sync.Mutex
	steps []Step
}

// New installs a Script as prompt.Default for the duration of the test.
func New(t testing.TB, steps ...Step) *Script {
	t.Helper()
	s := &Script{t: t, steps: steps}
	orig := prompt.Default
	prompt.Default = s
	t.Cleanup(func() {
		prompt.Default = orig
		if len(s.steps) > 0 {
			t.Errorf("prompttest: %d unused step(s), next: %s %q", len(s.steps), s.steps[0].kind, s.steps[0].contains)
		}
	})
	return s
}

// next pops the next step, failing the test if it doesn't match the prompt.
// It returns ok=false for a Cancel step.
func (s *Script) next(kind, msg string) (Step, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.steps) == 0 {
		s.t.Fatalf("prompttest: unexpected %s prompt %q", kind, msg)
	}
	st := s.steps[0]
	s.steps = s.steps[1:]
	if !strings.Contains(msg, st.contains) {
		s.t.Fatalf("prompttest: got %s prompt %q, want message containing %q", kind, msg, st.contains)
	}
	if st.kind == "cancel" {
		return st, false
	}
	if st.kind != kind {
		s.t.Fatalf("prompttest: got %s prompt %q, want %s", kind, msg, st.kind)
	}
	return st, true
}

// Select implements prompt.Prompter.
func (s *Script) Select(msg string, options []string, o prompt.Options) (int, error) {
	st, ok := s.next("select", msg)
	if !ok {
		return -1, prompt.ErrCanceled
	}
	answer := st.answer.(string)
	if answer == "" {
		answer = o.Default
	}
	i := slices.Index(options, answer)
	if i < 0 {
		s.t.Fatalf("prompttest: %q is not an option of %q (options: %v)", answer, msg, options)
	}
	return i, nil
}

// MultiSelect implements prompt.Prompter.
func (s *Script) MultiSelect(msg string, options []string) ([]int, error) {
	st, ok := s.next("multiselect", msg)
	if !ok {
		return nil, prompt.ErrCanceled
	}
	var idx []int
	for _, a := range st.answer.([]string) {
		i := slices.Index(options, a)
		if i < 0 {
			s.t.Fatalf("prompttest: %q is not an option of %q (options: %v)", a, msg, options)
		}
		idx = append(idx, i)
	}
	return idx, nil
}

// Input implements prompt.Prompter.
func (s *Script) Input(msg string, o prompt.Options) (string, error) {
	return s.text("input", msg, o)
}

// Secret implements prompt.Prompter.
func (s *Script) Secret(msg string, o prompt.Options) (string, error) {
	o.Default = ""
	return s.text("secret", msg, o)
}

func (s *Script) text(kind, msg string, o prompt.Options) (string, error) {
	st, ok := s.next(kind, msg)
	if !ok {
		return "", prompt.ErrCanceled
	}
	answer := st.answer.(string)
	if answer == "" {
		answer = o.Default
	}
	if o.Validate != nil {
		if err := o.Validate(answer); err != nil {
			s.t.Fatalf("prompttest: answer %q to %q fails validation: %v", answer, msg, err)
		}
	}
	return answer, nil
}

// Confirm implements prompt.Prompter.
func (s *Script) Confirm(msg string, _ bool) (bool, error) {
	st, ok := s.next("confirm", msg)
	if !ok {
		return false, prompt.ErrCanceled
	}
	return st.answer.(bool), nil
}
