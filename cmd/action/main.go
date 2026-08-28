package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	seleneDefaultVariant     = "selene"
	initialOutputBufferBytes = 64 * 1024
	maxSeleneOutputLineBytes = 1 << 20
)

type Config struct {
	WorkingDirectory    string
	ConfigPath          string
	LintPath            string
	SeleneArgs          string
	FailOnWarnings      bool
	ReportAsAnnotations bool
	SeleneVariant       string
	GithubWorkspace     string
}

type Annotation struct {
	File    string
	Line    string
	EndLine string
	Title   string
	Message string
	Level   string
}

type SelenePrimaryLabel struct {
	Filename string `json:"filename"`
	Span     struct {
		StartLine   int `json:"start_line"`
		StartColumn int `json:"start_column"`
	} `json:"span"`
}

type SeleneFinding struct {
	Severity     string             `json:"severity"`
	Code         string             `json:"code"`
	Message      string             `json:"message"`
	PrimaryLabel SelenePrimaryLabel `json:"primary_label"`
}

func main() {
	cfg, err := loadConfig(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
		os.Exit(1)
	}

	selenePath, err := resolveSelene(cfg.SeleneVariant)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error ensuring Selene: %v\n", err)
		os.Exit(1)
	}

	if err := runLinter(selenePath, cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Action failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Selene linting action completed successfully according to configuration.")
}

func loadConfig(args []string) (*Config, error) {
	flags := flag.NewFlagSet("selene-lua-linter-action", flag.ContinueOnError)
	workingDirectory := flags.String("working-directory", getInput("INPUT_WORKING-DIRECTORY", "."), "")
	configPath := flags.String("config-path", getInput("INPUT_CONFIG-PATH", ""), "")
	lintPath := flags.String("lint-path", getInput("INPUT_LINT-PATH", "."), "")
	seleneArgs := flags.String("selene-args", getInput("INPUT_SELENE-ARGS", ""), "")
	failOnWarningsInput := flags.String("fail-on-warnings", getInput("INPUT_FAIL-ON-WARNINGS", "false"), "")
	reportAsAnnotationsInput := flags.String(
		"report-as-annotations",
		getInput("INPUT_REPORT-AS-ANNOTATIONS", "true"),
		"",
	)
	seleneVariant := flags.String("selene-variant", getInput("INPUT_SELENE-VARIANT", seleneDefaultVariant), "")
	if err := flags.Parse(args); err != nil {
		return nil, err
	}
	if flags.NArg() != 0 {
		return nil, fmt.Errorf("unexpected arguments: %s", strings.Join(flags.Args(), " "))
	}
	failOnWarnings, err := strconv.ParseBool(*failOnWarningsInput)
	if err != nil {
		return nil, fmt.Errorf("fail-on-warnings must be true or false")
	}
	reportAsAnnotations, err := strconv.ParseBool(*reportAsAnnotationsInput)
	if err != nil {
		return nil, fmt.Errorf("report-as-annotations must be true or false")
	}
	cfg := &Config{
		WorkingDirectory:    *workingDirectory,
		ConfigPath:          *configPath,
		LintPath:            *lintPath,
		SeleneArgs:          *seleneArgs,
		FailOnWarnings:      failOnWarnings,
		ReportAsAnnotations: reportAsAnnotations,
		SeleneVariant:       *seleneVariant,
		GithubWorkspace:     os.Getenv("GITHUB_WORKSPACE"),
	}

	if cfg.ConfigPath == "" {
		return nil, fmt.Errorf("config-path is required")
	}
	if cfg.GithubWorkspace == "" {
		return nil, fmt.Errorf("GITHUB_WORKSPACE is required")
	}
	if !filepath.IsAbs(cfg.WorkingDirectory) {
		cfg.WorkingDirectory = filepath.Join(cfg.GithubWorkspace, cfg.WorkingDirectory)
	}
	if cfg.ConfigPath != "" {
		if !filepath.IsAbs(cfg.ConfigPath) {
			cfg.ConfigPath = filepath.Join(cfg.WorkingDirectory, cfg.ConfigPath)
			cfg.ConfigPath = filepath.Clean(cfg.ConfigPath)
		}
	}

	return cfg, nil
}

func getInput(name, defaultValue string) string {
	val := os.Getenv(name)
	if val == "" {
		return defaultValue
	}
	return val
}

func resolveSelene(variant string) (string, error) {
	if variant != "selene" && variant != "selene-light" {
		return "", fmt.Errorf("selene-variant must be selene or selene-light")
	}
	path := filepath.Join("/usr/local/bin", variant)
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("preinstalled %s is unavailable: %w", variant, err)
	}
	return path, nil
}

func runLinter(selenePath string, cfg *Config) error {
	args := []string{}
	if cfg.ConfigPath != "" {
		args = append(args, "--config", cfg.ConfigPath)
	}

	seleneArgsFields := strings.Fields(cfg.SeleneArgs)
	hasDisplayStyle := false
	for i, field := range seleneArgsFields {
		if field == "--display-style" {
			hasDisplayStyle = true
			if i+1 >= len(seleneArgsFields) {
			}
			break
		}
	}

	if !hasDisplayStyle {
		args = append(args, "--display-style", "Json")
	}

	if cfg.SeleneArgs != "" {
		args = append(args, seleneArgsFields...)
	}

	args = append(args, cfg.LintPath)

	fmt.Printf("Executing Selene in directory: %s\n", cfg.WorkingDirectory)
	fmt.Printf("Selene command: %s %s\n", selenePath, strings.Join(args, " "))

	cmd := exec.Command(selenePath, args...)
	cmd.Dir = cfg.WorkingDirectory

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}
	cmd.Stderr = os.Stderr

	if err = cmd.Start(); err != nil {
		return fmt.Errorf("failed to start Selene: %w", err)
	}

	var annotations []Annotation

	scanner := bufio.NewScanner(stdoutPipe)
	scanner.Buffer(make([]byte, initialOutputBufferBytes), maxSeleneOutputLineBytes)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "{") || !strings.HasSuffix(line, "}") {
			fmt.Println(line)
			continue
		}

		fmt.Println(line)

		if cfg.ReportAsAnnotations {
			var finding SeleneFinding
			jsonErr := json.Unmarshal([]byte(line), &finding)
			if jsonErr != nil {
				fmt.Fprintf(os.Stderr, "Warning: Failed to unmarshal Selene JSON line for annotation: %v. Line: %s\n", jsonErr, line)
				continue
			}

			levelString := strings.ToLower(finding.Severity)
			ruleName := finding.Code
			messageContent := finding.Message

			level := "notice"
			if levelString == "error" {
				level = "error"
			} else if levelString == "warning" {
				level = "warning"
			}

			title := fmt.Sprintf("Selene %s (%s)", finding.Severity, ruleName)

			filePath := finding.PrimaryLabel.Filename
			if !filepath.IsAbs(filePath) {
				filePath = filepath.Join(cfg.WorkingDirectory, filePath)
			}

			relPath, errRel := filepath.Rel(cfg.GithubWorkspace, filePath)
			if errRel == nil {
				filePath = relPath
			} else {
				fmt.Fprintf(os.Stderr, "Warning: Could not make path %s relative to GITHUB_WORKSPACE (%s): %v. Using original path for annotation.\n", filePath, cfg.GithubWorkspace, errRel)
			}

			annotations = append(annotations, Annotation{
				File:    filePath,
				Line:    fmt.Sprintf("%d", finding.PrimaryLabel.Span.StartLine),
				Title:   title,
				Message: messageContent,
				Level:   level,
			})
		}
	}
	scanErr := scanner.Err()

	seleneCmdErr := cmd.Wait()
	seleneExitCode := 0

	if seleneCmdErr != nil {
		if exitErr, ok := seleneCmdErr.(*exec.ExitError); ok {
			seleneExitCode = exitErr.ExitCode()
		} else {
			fmt.Fprintf(os.Stderr, "Selene command execution failed (not an exit code error): %v\n", seleneCmdErr)
			return seleneCmdErr
		}
	} else {
		seleneExitCode = 0
	}

	if cfg.ReportAsAnnotations && len(annotations) > 0 {
		for _, ann := range annotations {
			fmt.Printf("::%s file=%s,line=%s,title=%s::%s\n",
				ann.Level,
				workflowEscapeProperty(ann.File),
				ann.Line,
				workflowEscapeProperty(ann.Title),
				workflowEscapeData(ann.Message),
			)
		}
	}
	if scanErr != nil {
		return fmt.Errorf("failed to read Selene output: %w", scanErr)
	}

	hasFailureAnnotations := false
	for _, ann := range annotations {
		if ann.Level == "error" {
			hasFailureAnnotations = true
			break
		}
	}

	if hasFailureAnnotations {
		fmt.Fprintf(os.Stderr, "Selene reported critical errors (e.g., parse errors) via annotations. Action will fail.\n")
		if seleneCmdErr != nil {
			return fmt.Errorf("Selene reported critical errors (see annotations). Selene error: %w", seleneCmdErr)
		}
		return fmt.Errorf("Selene reported critical errors (see annotations)")
	}

	if seleneExitCode == 0 {
		fmt.Println("Selene exited with code 0.")
		hasLintingWarningsInAnnotations := false
		for _, ann := range annotations {
			if ann.Level == "warning" {
				hasLintingWarningsInAnnotations = true
				break
			}
		}
		if cfg.FailOnWarnings && hasLintingWarningsInAnnotations {
			fmt.Fprintf(os.Stderr, "Selene reported warnings (via annotations), and 'fail-on-warnings' is true. Action will fail.\n")
			return fmt.Errorf("Selene exited 0 but reported warnings, and 'fail-on-warnings' is true")
		}
		fmt.Println("Action successful.")
		return nil
	} else if seleneExitCode == 1 {
		fmt.Printf("Selene exited with code 1 (warnings reported).\n")
		if cfg.FailOnWarnings {
			fmt.Fprintf(os.Stderr, "'fail-on-warnings' is true. Action will fail.\n")
			return fmt.Errorf("Selene reported warnings (exit code 1), and 'fail-on-warnings' is true. Selene error: %w", seleneCmdErr)
		}
		fmt.Println("'fail-on-warnings' is false. Action successful despite warnings.")
		return nil
	} else {
		fmt.Printf("Selene exited with a critical error code: %d.\n", seleneExitCode)
		fmt.Fprintf(os.Stderr, "Action will fail.\n")
		return fmt.Errorf("Selene exited with a critical error code: %d. Selene error: %w", seleneExitCode, seleneCmdErr)
	}
}

func workflowEscapeData(value string) string {
	value = strings.ReplaceAll(value, "%", "%25")
	value = strings.ReplaceAll(value, "\r", "%0D")
	return strings.ReplaceAll(value, "\n", "%0A")
}

func workflowEscapeProperty(value string) string {
	value = workflowEscapeData(value)
	value = strings.ReplaceAll(value, ":", "%3A")
	return strings.ReplaceAll(value, ",", "%2C")
}
