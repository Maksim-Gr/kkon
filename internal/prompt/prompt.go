// Package prompt asks the user questions on the terminal. Commands call the
// package-level functions, which go through Default so tests can swap in a
// scripted Prompter (see prompttest).
package prompt

import (
	"errors"
	"fmt"
	"io"
	"slices"

	"github.com/AlecAivazis/survey/v2"
	"github.com/AlecAivazis/survey/v2/terminal"
)

// ErrCanceled is returned when the user interrupts a prompt (Ctrl+C or EOF).
var ErrCanceled = errors.New("canceled")

// Options tunes a single prompt. Validate is ignored by Select.
type Options struct {
	Default  string
	Help     string
	Validate func(string) error
}

// Prompter asks one question per call. Every method returns ErrCanceled when
// the user interrupts, and wraps any other failure as "prompt failed".
type Prompter interface {
	// Select returns the index of the chosen option. A Default that is not
	// one of the options is ignored.
	Select(msg string, options []string, o Options) (int, error)
	// MultiSelect returns the indexes of the chosen options; at least one
	// must be chosen.
	MultiSelect(msg string, options []string) ([]int, error)
	// Input returns the typed answer, or Default when the answer is empty.
	Input(msg string, o Options) (string, error)
	// Secret is Input with the answer masked; Default is ignored.
	Secret(msg string, o Options) (string, error)
	Confirm(msg string, def bool) (bool, error)
}

// Default is the Prompter the package-level functions use.
var Default Prompter = Survey{}

// Select asks Default to choose one option.
func Select(msg string, options []string, o Options) (int, error) {
	return Default.Select(msg, options, o)
}

// MultiSelect asks Default to choose one or more options.
func MultiSelect(msg string, options []string) ([]int, error) {
	return Default.MultiSelect(msg, options)
}

// Input asks Default for a line of text.
func Input(msg string, o Options) (string, error) {
	return Default.Input(msg, o)
}

// Secret asks Default for a masked line of text.
func Secret(msg string, o Options) (string, error) {
	return Default.Secret(msg, o)
}

// Confirm asks Default a yes/no question.
func Confirm(msg string, def bool) (bool, error) {
	return Default.Confirm(msg, def)
}

// Required is a Validate func that rejects an empty answer.
func Required(s string) error {
	if s == "" {
		return errors.New("Value is required") //nolint:staticcheck // matches survey's wording
	}
	return nil
}

// Survey is the terminal Prompter backed by github.com/AlecAivazis/survey.
type Survey struct{}

// Select implements Prompter.
func (Survey) Select(msg string, options []string, o Options) (int, error) {
	p := &survey.Select{Message: msg, Options: options, Help: o.Help, PageSize: 10}
	if slices.Contains(options, o.Default) {
		p.Default = o.Default
	}
	var ans survey.OptionAnswer
	if err := survey.AskOne(p, &ans); err != nil {
		return -1, wrap(err)
	}
	return ans.Index, nil
}

// MultiSelect implements Prompter.
func (Survey) MultiSelect(msg string, options []string) ([]int, error) {
	var ans []survey.OptionAnswer
	p := &survey.MultiSelect{Message: msg, Options: options}
	if err := survey.AskOne(p, &ans, survey.WithValidator(survey.MinItems(1))); err != nil {
		return nil, wrap(err)
	}
	idx := make([]int, len(ans))
	for i, a := range ans {
		idx[i] = a.Index
	}
	return idx, nil
}

// Input implements Prompter.
func (Survey) Input(msg string, o Options) (string, error) {
	var ans string
	err := survey.AskOne(&survey.Input{Message: msg, Default: o.Default, Help: o.Help}, &ans, validator(o.Validate)...)
	return ans, wrap(err)
}

// Secret implements Prompter.
func (Survey) Secret(msg string, o Options) (string, error) {
	var ans string
	err := survey.AskOne(&survey.Password{Message: msg, Help: o.Help}, &ans, validator(o.Validate)...)
	return ans, wrap(err)
}

// Confirm implements Prompter.
func (Survey) Confirm(msg string, def bool) (bool, error) {
	var ans bool
	err := survey.AskOne(&survey.Confirm{Message: msg, Default: def}, &ans)
	return ans, wrap(err)
}

func validator(v func(string) error) []survey.AskOpt {
	if v == nil {
		return nil
	}
	return []survey.AskOpt{survey.WithValidator(func(ans any) error {
		s, _ := ans.(string)
		return v(s)
	})}
}

func wrap(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, terminal.InterruptErr), errors.Is(err, io.EOF):
		return ErrCanceled
	default:
		return fmt.Errorf("prompt failed: %w", err)
	}
}
