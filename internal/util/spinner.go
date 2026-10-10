package util

import (
	"fmt"
	"os"
	"time"
)

// StartSpinner starts a simple terminal spinner with msg and returns a stop function.
// Call the returned function to stop the spinner and clear the line.
// Frames are written to stderr so piped stdout (e.g. --output json) stays clean,
// and nothing is drawn when stderr is not a terminal.
func StartSpinner(msg string) func() {
	if !IsTerminal(os.Stderr) {
		return func() {}
	}
	done := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		frames := []rune{'⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇', '⠏'}
		i := 0
		for {
			select {
			case <-done:
				fmt.Fprint(os.Stderr, "\r\033[K")
				return
			default:
				fmt.Fprintf(os.Stderr, "\r%s %c", msg, frames[i%len(frames)])
				i++
				time.Sleep(80 * time.Millisecond)
			}
		}
	}()
	return func() {
		close(done)
		<-stopped
	}
}
