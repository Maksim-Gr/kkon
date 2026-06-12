package config

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gokafkaconnect/internal/connector"
	"gokafkaconnect/internal/util"

	"github.com/AlecAivazis/survey/v2"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var dryRun bool

// ConfigureCmd represents the configure command.
var ConfigureCmd = &cobra.Command{
	Use:   "set",
	Short: "Configure Kafka Connect REST API",
	Long:  `Configure Kafka Connect REST API URL and authentication.`,
	RunE: func(_ *cobra.Command, _ []string) error {

		if dryRun {
			color.Cyan("Dry run mode")
		} else {
			color.Cyan("\nConfiguring Kafka Connect...\n")
		}

		configPath, err := util.GetConfigPath()
		if err != nil {
			return fmt.Errorf("failed to determine config path: %w", err)
		}

		currentURL := ""
		currentUser := ""
		currentPass := ""

		if loaded, err := util.LoadConfig(); err == nil {
			currentURL = loaded.KafkaConnect.URL
			currentUser = loaded.KafkaConnect.Username
			currentPass = loaded.KafkaConnect.Password
			color.Yellow("Current Kafka Connect URL: %s", currentURL)
		}

		// --- URL prompt ---
		var inputURL string
		urlPrompt := &survey.Input{
			Message: "Kafka Connect URL:",
			Help:    "Enter the URL of your Kafka Connect REST API (e.g. http://localhost:8083)",
			Default: currentURL,
		}

		err = survey.AskOne(urlPrompt, &inputURL, survey.WithValidator(
			func(ans interface{}) error {
				s := ans.(string)

				if s == currentURL {
					return nil
				}
				if s == "" && currentURL == "" {
					return errors.New("URL cannot be empty")
				}
				if s == "" {
					return nil
				}
				return util.ValidateURL(s)
			},
		))
		if err != nil {
			if util.IsSurveyInterrupt(err) {
				color.Yellow("Canceled\n")
				return nil
			}
			return fmt.Errorf("failed to read URL: %w", err)
		}

		if inputURL == "" {
			inputURL = currentURL
		} else if !strings.HasPrefix(inputURL, "http://") && !strings.HasPrefix(inputURL, "https://") {
			color.Yellow("No scheme specified — assuming http://")
			inputURL = "http://" + inputURL
		}

		var inputUser string
		userPrompt := &survey.Input{
			Message: "Kafka Connect username (leave empty for no auth):",
			Default: currentUser,
		}

		if err := survey.AskOne(userPrompt, &inputUser); err != nil {
			if util.IsSurveyInterrupt(err) {
				color.Yellow("Canceled\n")
				return nil
			}
			return fmt.Errorf("failed to read username: %w", err)
		}

		// Ask for password only when a username is being set for the first time
		// or explicitly changed. If the user pressed Enter keeping the existing
		// username, retain the stored password without prompting.
		inputPass := currentPass
		if inputUser == "" {
			inputPass = ""
		} else if inputUser != currentUser {
			passPrompt := &survey.Password{
				Message: "Kafka Connect password:",
				Help:    "Password will be stored in your local config file",
			}
			if err := survey.AskOne(passPrompt, &inputPass); err != nil {
				if util.IsSurveyInterrupt(err) {
					color.Yellow("Canceled\n")
					return nil
				}
				return fmt.Errorf("failed to read password: %w", err)
			}
		}

		cfg := util.RestAPIConfig{
			KafkaConnect: util.KafkaConnectConfig{
				URL:      inputURL,
				Username: inputUser,
				Password: inputPass,
			},
		}

		if dryRun {
			color.Cyan("Dry run mode — config will not be saved.")
			color.Cyan("Kafka Connect URL: %s", inputURL)
			if inputUser != "" {
				color.Cyan("Authentication: enabled (username: %s)", inputUser)
			} else {
				color.Cyan("Authentication: disabled")
			}
			return nil
		}

		if err := util.SaveConfig(cfg, configPath); err != nil {
			return fmt.Errorf("failed to save config file: %w", err)
		}

		color.Green("Configuration saved successfully!")
		color.Green("Kafka Connect URL: %s", inputURL)
		if inputUser != "" {
			color.Green("Authentication enabled for user: %s", inputUser)
		} else {
			color.Green("Authentication disabled")
		}

		var testConn bool
		testPrompt := &survey.Confirm{
			Message: fmt.Sprintf("Test connection to %s?", inputURL),
			Default: true,
		}
		if err := survey.AskOne(testPrompt, &testConn); err == nil && testConn {
			stop := util.StartSpinner("Testing connection...")
			testClient := connector.NewClient(inputURL)
			if inputUser != "" {
				testClient.SetBasicAuth(inputUser, inputPass)
			}
			list, err := testClient.ListConnectors(context.Background())
			stop()
			if err != nil {
				color.Red("Connection failed: %v\n", err)
			} else {
				color.Green("Connection OK — %d connector(s) found\n", len(list))
			}
		}

		return nil
	},
}

// SetDryRun sets the dry-run flag for config subcommands.
func SetDryRun(value bool) {
	dryRun = value
}
