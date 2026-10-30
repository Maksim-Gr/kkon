package connector

import (
	"fmt"

	"github.com/AlecAivazis/survey/v2"
)

func displayConfigValue(key, value string) string {
	if value != "" && isSecretField(key) {
		return "********"
	}
	return value
}

// redactConfig returns a display copy; submissions and backups retain the values.
func redactConfig(cfg map[string]string) map[string]string {
	redacted := make(map[string]string, len(cfg))
	for key, value := range cfg {
		redacted[key] = displayConfigValue(key, value)
	}
	return redacted
}

func configValuePrompt(key string) survey.Prompt {
	message := fmt.Sprintf("New value for %s:", key)
	if isSecretField(key) {
		return &survey.Password{Message: message}
	}
	return &survey.Input{Message: message}
}
