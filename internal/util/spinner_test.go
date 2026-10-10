package util

import (
	"io"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStartSpinnerSilentWithoutTerminal(t *testing.T) {
	r, w, err := os.Pipe()
	require.NoError(t, err)
	orig := os.Stderr
	os.Stderr = w
	defer func() { os.Stderr = orig }()

	stop := StartSpinner("Working...")
	time.Sleep(200 * time.Millisecond)
	stop()
	require.NoError(t, w.Close())

	out, err := io.ReadAll(r)
	require.NoError(t, err)
	assert.Empty(t, string(out))
}
