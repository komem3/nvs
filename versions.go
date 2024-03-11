package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/net/html"
)

var versionsRemoteArg bool

var VersionsCmd = &cobra.Command{
	Use:   "versions",
	Short: "List version",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, _ []string) {
		ctx := cmd.Context()
		if versionsRemoteArg {
			if err := outputRemoteVersions(); err != nil {
				fatal(ctx, err)
			}
		} else {
			if err := outputLocalVersions(ctx); err != nil {
				fatal(ctx, err)
			}
		}
	},
}

func init() {
	VersionsCmd.Flags().BoolVar(&versionsRemoteArg, "remote", false, "list remote versions")
}

var versionRegex = regexp.MustCompile(`^v[0-9]{1,2}\.[0-9]{1,2}\.[0-9]{1,2}/$`)

func outputRemoteVersions() error {
	r, err := http.NewRequest(http.MethodGet, nodejsURL, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return fmt.Errorf("status is %d. response %s", resp.StatusCode, body)
	}

	doc, err := html.Parse(resp.Body)
	if err != nil {
		return err
	}
	var f func(*html.Node) error
	var buf strings.Builder
	f = func(n *html.Node) error {
		if n.Type == html.ElementNode && n.Data == "a" {
			data := n.FirstChild.Data
			if !versionRegex.MatchString(data) {
				return nil
			}
			buf.WriteString(strings.TrimRight(data, "/"))
			buf.WriteRune('\n')
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if err := f(c); err != nil {
				return err
			}
		}
		return nil
	}
	if err := f(doc); err != nil {
		return err
	}

	os.Stdout.WriteString(buf.String())

	return nil
}

func outputLocalVersions(ctx context.Context) error {
	baseDir, err := checkInit()
	if err != nil {
		return err
	}
	versions, err := os.ReadDir(filepath.Join(baseDir, "versions"))
	if err != nil {
		return err
	}

	getVersion := func(versionString string) (string, error) {
		if versionString == "" {
			return "", nil
		}
		parsedVersion, err := parseVersionString(versionString)
		if err != nil {
			return "", err
		}
		path, err := findLocalVersion(baseDir, parsedVersion)
		if err != nil {
			if errors.Is(err, ErrNotFoundLocalVersion) {
				if err := Download(ctx, parsedVersion); err != nil {
					return "", err
				}
				path, err = findLocalVersion(baseDir, parsedVersion)
				if err != nil {
					return "", err
				}
			} else {
				return "", err
			}
		}
		return filepath.Base(path), nil
	}

	current, err := decideVersion(ctx, baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			debugf(ctx, "local version is not found")
		} else {
			return err
		}
	}
	currentVersion, err := getVersion(current)
	if err != nil {
		return err
	}

	global, err := os.ReadFile(filepath.Join(baseDir, globalVersionFile))
	if err != nil {
		if os.IsNotExist(err) {
			debugf(ctx, "global version is not found")
		} else {
			return err
		}
	}
	globalVersion, err := getVersion(string(global))
	if err != nil {
		return err
	}

	var buf strings.Builder
	for _, version := range versions {
		name := version.Name()
		switch {
		case name == globalVersion && name == currentVersion:
			buf.WriteRune('*')
		case name == globalVersion:
			buf.WriteRune('-')
		case name == currentVersion:
			buf.WriteRune('+')
		default:
			buf.WriteRune(' ')
		}
		fmt.Fprintf(&buf, " %s\n", name)
	}
	buf.WriteString("\n-global +current *both\n")
	os.Stdout.WriteString(buf.String())

	return nil
}
