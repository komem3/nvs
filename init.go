package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var InitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize nvs",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		if err := Initialize(); err != nil {
			fatal(cmd.Context(), err)
		}
		fmt.Printf(`Initialize Success.
Add nvs to PATH

export PATH="$HOME/.nvs/bin:$PATH"

And, select global Nddejs version

nvs use 20
`)
	},
}

const nvsDir = ".nvs"

func checkInit() (dir string, err error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	dir = filepath.Join(home, nvsDir)
	if _, err := os.Stat(dir); err != nil {
		return "", err
	}
	return dir, nil
}

func Initialize() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	dir := filepath.Join(home, nvsDir)
	if err := os.Mkdir(dir, 0755); err != nil {
		if !os.IsExist(err) {
			return err
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "bin"), 0755); err != nil {
		if !os.IsExist(err) {
			return err
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "versions"), 0755); err != nil {
		if !os.IsExist(err) {
			return err
		}
	}

	if err := createScript(dir, "node"); err != nil {
		return fmt.Errorf("create node script: %w", err)
	}
	if err := createScript(dir, "npm"); err != nil {
		return fmt.Errorf("create npm script: %w", err)
	}
	if err := createScript(dir, "corepack"); err != nil {
		return fmt.Errorf("create corepack script: %w", err)
	}
	if err := createScript(dir, "npx"); err != nil {
		return fmt.Errorf("create npx script: %w", err)
	}
	return nil
}

func createScript(dir, command string) error {
	if err := os.WriteFile(filepath.Join(dir, "bin", command), []byte("#!/bin/bash\nnvs run "+command+" -- $@\n"), 0744); err != nil {
		return err
	}
	return nil
}
