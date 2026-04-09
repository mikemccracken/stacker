package main

import (
	"os"
	"regexp"
	"strings"

	"github.com/pkg/errors"
	cli "github.com/urfave/cli/v2"
	"stackerbuild.io/stacker/pkg/stacker"
)

var (
	credentialValuePatterns = []*regexp.Regexp{
		regexp.MustCompile(`^[^:\\s]+:[^\\s]{12,}$`),
		regexp.MustCompile(`^[A-Za-z0-9-_]+[.][A-Za-z0-9-_]+[.][A-Za-z0-9-_]+$`),
		regexp.MustCompile(`^(?i)akia[0-9a-z]{16}$`),
		regexp.MustCompile(`^[A-Fa-f0-9]{40,64}$`),
		regexp.MustCompile(`^[A-Za-z0-9+/=]{32,}$`),
	}
	credentialValueKeywords = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(password|passwd|pass|secret|token|credential|apikey|api[_-]?key|access[_-]?key|auth|private[_-]?key|cred)`),
	}
)

/*
	check that roots-dir./ name don't contain ':', it will interfere with overlay mount options

which is using :s as separator
*/
func validateRootsDirName(rootsDir string) error {
	if strings.Contains(rootsDir, ":") {
		return errors.Errorf("using ':' in the name of --roots-dir (%s) is forbidden due to overlay constraints", rootsDir)
	}

	return nil
}

func validateBuildFailureFlags(ctx *cli.Context) error {
	if ctx.Bool("shell-fail") {
		askedFor := ctx.String("on-run-failure")
		if askedFor != "" && askedFor != stacker.DefaultShell {
			return errors.Errorf("--shell-fail is incompatible with --on-run-failure=%s", askedFor)
		}
		err := ctx.Set("on-run-failure", stacker.DefaultShell)
		if err != nil {
			return err
		}
	}

	return nil
}

func validateLayerTypeFlags(ctx *cli.Context) error {
	layerTypes := ctx.StringSlice("layer-type")
	if len(layerTypes) == 0 {
		return errors.Errorf("must specify at least one output --layer-type")
	}

	for _, layerType := range layerTypes {
		switch layerType {
		case "tar":
			break
		case "squashfs":
			break
		case "erofs":
			break
		default:
			return errors.Errorf("unknown layer type: %s", layerType)
		}
	}

	return nil
}

func validateFileSearchFlags(ctx *cli.Context) error {
	// Use the current working directory if base search directory is "."
	if ctx.String("search-dir") == "." {
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		err = ctx.Set("search-dir", wd)
		if err != nil {
			return err
		}
	}

	// Ensure the base search directory exists
	if _, err := os.Lstat(ctx.String("search-dir")); err != nil {
		return err
	}

	// Ensure the stacker-file-pattern variable compiles as a regex
	if _, err := regexp.Compile(ctx.String("stacker-file-pattern")); err != nil {
		return err
	}

	return nil
}

func validateSubstituteFlags(subs []string) error {
	for _, sub := range subs {
		key, value, ok := strings.Cut(sub, "=")
		if !ok {
			return errors.Errorf("invalid substitution %s, expected KEY=value", sub)
		}
		if strings.TrimSpace(key) == "" {
			return errors.Errorf("invalid substitution %s, expected KEY=value", sub)
		}
		if looksLikeCredentialValue(value) || looksLikeCredentialKey(key, value) {
			return errors.Errorf("refusing substitution %q because it looks like a credential", sub)
		}
	}

	return nil
}

func looksLikeCredentialValue(value string) bool {
	if value == "" {
		return false
	}
	lowerValue := strings.ToLower(strings.TrimSpace(value))
	if strings.Contains(lowerValue, "://") {
		return false
	}
	for _, re := range credentialValuePatterns {
		if re.MatchString(value) {
			return true
		}
	}
	return false
}

func looksLikeCredentialKey(key string, value string) bool {
	for _, re := range credentialValueKeywords {
		if re.MatchString(key) && len(value) >= 12 {
			return true
		}
	}
	return false
}
