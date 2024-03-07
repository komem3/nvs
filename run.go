package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"
)

var (
	runVersionArg string
	runVerboseArg bool
)

var RunCmd = &cobra.Command{
	Use:   "run [node|npm|npx|corepack]",
	Short: "Run command(node, npm or npx)",
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		verbose = runVerboseArg
		command := args[0]
		if !slices.Contains([]string{"node", "npm", "npx", "corepack"}, command) {
			cmd.Usage()
			os.Exit(1)
		}
		var commandArgs []string
		if len(args) > 1 {
			commandArgs = args[1:]
		}
		if err := Run(cmd.Context(), runVersionArg, command, commandArgs); err != nil {
			var extErr *exec.ExitError
			if errors.As(err, &extErr) {
				os.Exit(extErr.ExitCode())
			} else if errors.Is(err, ErrNotFoundGlobalVersion) {
				fatal(cmd.Context(), fmt.Errorf("no specify version. Run `nvs use`"))
			} else {
				fatal(cmd.Context(), err)
			}
		}
	},
}

func init() {
	RunCmd.Flags().StringVar(&runVersionArg, "version", autoVersion, "specify running version")
	RunCmd.Flags().BoolVarP(&runVerboseArg, "verbose", "v", false, "output verbose")
}

const autoVersion = "auto"

var ErrNotFoundGlobalVersion = fmt.Errorf("not found global version")

func decideVersion(ctx context.Context, baseDir string) (string, error) {
	globalVersion := func() (string, error) {
		v, err := os.ReadFile(filepath.Join(baseDir, globalVersionFile))
		if err != nil {
			if os.IsNotExist(err) {
				return "", ErrNotFoundGlobalVersion
			}
			return "", err
		}
		return string(v), nil
	}
	for dir := "."; ; dir = filepath.Join("..", dir) {
		directory, err := filepath.Abs(dir)
		if err != nil {
			debugf(ctx, "get %s abs: %v", dir, err)
			return globalVersion()
		}

		if nodeVersionFile, err := os.Open(filepath.Join(directory, ".node-version")); err == nil {
			debugf(ctx, "use .node-version")
			b, err := io.ReadAll(nodeVersionFile)
			if err != nil {
				return "", err
			}
			return strings.TrimRight(string(b), "\n"), nil
		}
		if packageFile, err := os.Open(filepath.Join(directory, "package.json")); err == nil {
			debugf(ctx, "parse package.json")
			var packageJson struct {
				Engines struct {
					Node string `json:"node"`
				} `json:"engines"`
			}
			if err := json.NewDecoder(packageFile).Decode(&packageJson); err != nil {
				return "", err
			}
			if packageJson.Engines.Node != "" {
				return packageJson.Engines.Node, nil
			}
		}
		if directory == "/" {
			return globalVersion()
		}
	}
}

func Run(ctx context.Context, versionStr string, command string, args []string) error {
	baseDir, err := checkInit()
	if err != nil {
		return err
	}

	var parsedVersion *version
	if versionStr != autoVersion {
		parsedVersion, err = parseVersionString(versionStr)
		if err != nil {
			return err
		}
	} else {
		versionStr, err = decideVersion(ctx, baseDir)
		if err != nil {
			return err
		}
		parsedVersion, err = parseVersionString(versionStr)
		if err != nil {
			return err
		}
	}
	nodeBasePath, err := findLocalVersion(baseDir, parsedVersion)
	if err != nil {
		if errors.Is(err, ErrNotFoundLocalVersion) {
			warnf(ctx, "download %s version", versionStr)
			if err := Download(ctx, parsedVersion); err != nil {
				return err
			}
			nodeBasePath, err = findLocalVersion(baseDir, parsedVersion)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}
	debugf(ctx, "use %s", nodeBasePath)

	if command != "node" {
		args = slices.Concat([]string{filepath.Join(baseDir, "versions", nodeBasePath, "bin", command)}, args)
	}
	cmd := exec.CommandContext(ctx, filepath.Join(baseDir, "versions", nodeBasePath, "bin", "node"), args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return err
	}
	if err := cmd.Wait(); err != nil {
		return err
	}
	return nil
}

type localFile struct {
	name     string
	priority int
}

var ErrNotFoundLocalVersion = fmt.Errorf("not found local nodejs")

func findLocalVersion(baseDir string, v *version) (string, error) {
	files, err := os.ReadDir(filepath.Join(baseDir, "versions"))
	if err != nil {
		return "", err
	}
	var matchFiles []localFile
	for _, file := range files {
		splitName := strings.Split(strings.TrimLeft(filepath.Base(file.Name()), "v"), ".")
		match, err := compareVersionString(splitName, v)
		if err != nil {
			return "", err
		}
		if match {
			matchFiles = append(matchFiles, localFile{
				name:     file.Name(),
				priority: calcPriority(splitName),
			})
		}
	}
	if len(matchFiles) == 0 {
		return "", ErrNotFoundLocalVersion
	}
	slices.SortFunc(matchFiles, func(l, r localFile) int {
		return r.priority - l.priority
	})
	return matchFiles[0].name, nil
}
