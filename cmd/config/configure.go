package config

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Maksim-Gr/kkon/internal/connector"
	"github.com/Maksim-Gr/kkon/internal/prompt"
	"github.com/Maksim-Gr/kkon/internal/util"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

// ConfigureCmd represents the configure command.
var ConfigureCmd = &cobra.Command{
	Use:   "set",
	Short: "Configure the Kafka Connect connection",
	Long: `Interactively set the Kafka Connect REST API URL and optional basic-auth
credentials, then optionally test the connection. Runs automatically the
first time kkon needs a connection.`,
	Example: `  # Run the configuration wizard
  kkon config set

  # Preview without saving the config
  kkon config set --dry-run`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		dryRun := util.IsDryRun(cmd)

		if dryRun {
			color.Cyan("Dry run mode")
		} else {
			color.Cyan("\nConfiguring Kafka Connect...\n")
		}

		configPath, err := util.GetConfigPath()
		if err != nil {
			return fmt.Errorf("failed to determine config path: %w", err)
		}

		// Start from the saved config so the Schema Registry section survives the save.
		cfg, loadErr := util.LoadConfig()
		if loadErr != nil {
			cfg = util.RestAPIConfig{}
		}
		currentURL := cfg.KafkaConnect.URL
		currentUser := cfg.KafkaConnect.Username
		currentPass := cfg.KafkaConnect.Password
		if loadErr == nil {
			color.Yellow("Current Kafka Connect URL: %s", currentURL)
		}

		// --- URL prompt ---
		inputURL, err := prompt.Input("Kafka Connect URL:", prompt.Options{
			Help:    "Enter the URL of your Kafka Connect REST API (e.g. http://localhost:8083)",
			Default: currentURL,
			Validate: func(s string) error {
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
		})
		if err != nil {
			return err
		}

		if inputURL == "" {
			inputURL = currentURL
		} else if !strings.HasPrefix(inputURL, "http://") && !strings.HasPrefix(inputURL, "https://") {
			color.Yellow("No scheme specified — assuming http://")
			inputURL = "http://" + inputURL
		}

		inputUser, err := prompt.Input("Kafka Connect username (leave empty for no auth):", prompt.Options{Default: currentUser})
		if err != nil {
			return err
		}

		// Ask for password only when a username is being set for the first time
		// or explicitly changed. If the user pressed Enter keeping the existing
		// username, retain the stored password without prompting.
		inputPass := currentPass
		if inputUser == "" {
			inputPass = ""
		} else if inputUser != currentUser {
			if inputPass, err = prompt.Secret("Kafka Connect password:", prompt.Options{
				Help: "Password will be stored in your local config file",
			}); err != nil {
				return err
			}
		}

		cfg.KafkaConnect = util.KafkaConnectConfig{
			URL:      inputURL,
			Username: inputUser,
			Password: inputPass,
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

		testConn, err := prompt.Confirm(fmt.Sprintf("Test connection to %s?", inputURL), true)
		if err != nil {
			return err
		}
		if testConn {
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

		return configureSchemaRegistry(dryRun)
	},
}

// Skipped by default so existing Kafka-Connect-only users see no new
// required prompts.
func configureSchemaRegistry(dryRun bool) error {
	cfg, err := util.LoadConfig()
	if err != nil {
		cfg = util.RestAPIConfig{}
	}
	currentURL := cfg.SchemaRegistry.URL

	configureSR, err := prompt.Confirm("Configure Schema Registry too?", currentURL != "")
	if err != nil {
		return err
	}
	if !configureSR {
		return nil
	}

	inputURL, err := prompt.Input("Schema Registry URL:", prompt.Options{
		Help:    "Enter the URL of your Confluent Schema Registry (e.g. http://localhost:8081)",
		Default: currentURL,
		Validate: func(s string) error {
			if s == "" {
				return errors.New("URL cannot be empty")
			}
			return util.ValidateURL(s)
		},
	})
	if err != nil {
		return err
	}
	if !strings.HasPrefix(inputURL, "http://") && !strings.HasPrefix(inputURL, "https://") {
		color.Yellow("No scheme specified — assuming http://")
		inputURL = "http://" + inputURL
	}

	if dryRun {
		color.Cyan("Dry run mode — config will not be saved.")
		color.Cyan("Schema Registry URL: %s", inputURL)
		return nil
	}

	cfg.SchemaRegistry = util.SchemaRegistryConfig{URL: inputURL}

	configPath, err := util.GetConfigPath()
	if err != nil {
		return fmt.Errorf("failed to determine config path: %w", err)
	}
	if err := util.SaveConfig(cfg, configPath); err != nil {
		return fmt.Errorf("failed to save config file: %w", err)
	}

	color.Green("Schema Registry URL: %s", inputURL)
	return nil
}
