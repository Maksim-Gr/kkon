package prompt

import (
	"errors"
	"io"
	"testing"

	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWrap(t *testing.T) {
	assert.NoError(t, wrap(nil))
	assert.ErrorIs(t, wrap(terminal.InterruptErr), ErrCanceled)
	assert.ErrorIs(t, wrap(io.EOF), ErrCanceled)

	boom := errors.New("boom")
	err := wrap(boom)
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrCanceled)
	assert.ErrorIs(t, err, boom)
	assert.Contains(t, err.Error(), "prompt failed")
}
