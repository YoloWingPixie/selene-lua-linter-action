package main

import (
	"archive/zip"
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	seleneDefaultRepo    = "Kampfkarren/selene"
	seleneDefaultVariant = "selene"
	seleneDefaultVersion = "latest"
)

type Config struct {
	WorkingDirectory    string
	ConfigPath          string
	LintPath            string
	SeleneArgs          string
	FailOnWarnings      bool
	ReportAsAnnotations bool
	SeleneVersion       string
	SeleneRepo          string
	SeleneVariant       string
	GithubWorkspace     string
	GithubToken         string
}

type Annotation struct {
	File    string
	Line    string
	EndLine string
	Title   string
	Message string
	Level   string // "notice", "warning", or "failure"
}

type SelenePrimaryLabel struct {
	Filename string `json:"filename"`
	Span     struct {
		StartLine   int `json:"start_line"`
		StartColumn int `json:"start_column"`
	} `json:"span"`
}

type SeleneFinding struct {
	Severity     string             `json:"severity"` // "Warning", "Error"
	Code         string             `json:"code"`     // e.g., "global_usage"
	Message      string             `json:"message"`
	PrimaryLabel SelenePrimaryLabel `json:"primary_label"`
}

func main() {
	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
		os.Exit(1)
	}

	selenePath, err := ensureSelene(cfg.SeleneVersion, cfg.SeleneRepo, cfg.SeleneVariant)
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

func loadConfig() (*Config, error) {
	cfg := &Config{
		WorkingDirectory:    getInput("INPUT_WORKING-DIRECTORY", "."),
		ConfigPath:          getInput("INPUT_CONFIG-PATH", ""),
		LintPath:            getInput("INPUT_LINT-PATH", "."),
		SeleneArgs:          getInput("INPUT_SELENE-ARGS", ""),
		FailOnWarnings:      strings.ToLower(getInput("INPUT_FAIL-ON-WARNINGS", "false")) == "true",
		ReportAsAnnotations: strings.ToLower(getInput("INPUT_REPORT-AS-ANNOTATIONS", "true")) == "true",
		SeleneVersion:       getInput("INPUT_SELENE-VERSION", seleneDefaultVersion),
		SeleneRepo:          getInput("INPUT_SELENE-REPO", seleneDefaultRepo),
		SeleneVariant:       getInput("INPUT_SELENE-VARIANT", seleneDefaultVariant),
		GithubWorkspace:     os.Getenv("GITHUB_WORKSPACE"),
		GithubToken:         os.Getenv("INPUT_GITHUB-TOKEN"),
	}

	if cfg.ConfigPath == "" {
	}
	// Ensure paths are absolute or relative to GITHUB_WORKSPACE
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

func ensureSelene(version, repo, variant string) (string, error) {
	seleneBinaryPath := "/usr/local/bin/selene"

	_, statErr := os.Stat(seleneBinaryPath)

	if version != seleneDefaultVersion || (version == seleneDefaultVersion && os.IsNotExist(statErr)) {
		fmt.Printf("Attempting to download Selene %s (%s variant) from %s...\n", version, variant, repo)

		var downloadURL string
		var assetToDownloadName string

		if version == "latest" {
			apiURL := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)
			resp, httpErr := http.Get(apiURL)
			if httpErr != nil {
				return "", fmt.Errorf("failed to fetch latest release info from %s: %w", apiURL, httpErr)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				bodyBytes, _ := io.ReadAll(resp.Body)
				return "", fmt.Errorf("failed to fetch latest release info from %s, status: %s, body: %s", apiURL, resp.Status, string(bodyBytes))
			}

			var releaseInfo struct {
				Assets []struct {
					Name               string `json:"name"`
					BrowserDownloadURL string `json:"browser_download_url"`
				} `json:"assets"`
			}
			if decodeErr := decodeJSONBody(resp, &releaseInfo); decodeErr != nil {
				return "", fmt.Errorf("failed to decode release info: %w", decodeErr)
			}

			foundAsset := false
			expectedSuffix := "-linux.zip"
			for _, asset := range releaseInfo.Assets {
				if strings.HasPrefix(asset.Name, variant+"-") && strings.HasSuffix(asset.Name, expectedSuffix) {
					downloadURL = asset.BrowserDownloadURL
					assetToDownloadName = asset.Name
					foundAsset = true
					break
				}
			}
			if !foundAsset {
				return "", fmt.Errorf("could not find asset matching %s-VERSION%s for %s in latest release", variant, expectedSuffix, repo)
			}
		} else {
			versionStr := strings.TrimPrefix(version, "v")
			assetToDownloadName = fmt.Sprintf("%s-%s-linux.zip", variant, versionStr)
			downloadURL = fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", repo, version, assetToDownloadName)
		}

		fmt.Printf("Identified Selene asset for download: %s\n", assetToDownloadName)
		fmt.Printf("Downloading from URL: %s\n", downloadURL)

		resp, httpErr := http.Get(downloadURL)
		if httpErr != nil {
			return "", fmt.Errorf("failed to download Selene from %s: %w", downloadURL, httpErr)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			return "", fmt.Errorf("failed to download Selene from %s, status: %s, body: %s", downloadURL, resp.Status, string(bodyBytes))
		}

		tmpZipFile, err := os.CreateTemp("", "selene-*.zip")
		if err != nil {
			return "", fmt.Errorf("failed to create temporary zip file: %w", err)
		}
		defer os.Remove(tmpZipFile.Name())

		_, err = io.Copy(tmpZipFile, resp.Body)
		if err != nil {
			tmpZipFile.Close()
			return "", fmt.Errorf("failed to write Selene zip to temporary file: %w", err)
		}
		tmpZipFile.Close()

		zipReader, err := zip.OpenReader(tmpZipFile.Name())
		if err != nil {
			return "", fmt.Errorf("failed to open downloaded zip file %s: %w", tmpZipFile.Name(), err)
		}
		defer zipReader.Close()

		var seleneZipFile *zip.File
		for _, f := range zipReader.File {
			if f.Name == variant {
				seleneZipFile = f
				break
			}
			if filepath.Base(f.Name) == variant && !f.FileInfo().IsDir() {
				seleneZipFile = f
				break
			}
		}

		if seleneZipFile == nil {
			return "", fmt.Errorf("could not find '%s' binary within the downloaded zip %s", variant, assetToDownloadName)
		}

		srcFile, err := seleneZipFile.Open()
		if err != nil {
			return "", fmt.Errorf("failed to open '%s' from zip: %w", seleneZipFile.Name, err)
		}
		defer srcFile.Close()

		dstFile, err := os.OpenFile(seleneBinaryPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, seleneZipFile.Mode())
		if err != nil {
			return "", fmt.Errorf("failed to create destination file %s: %w", seleneBinaryPath, err)
		}
		defer dstFile.Close()

		_, err = io.Copy(dstFile, srcFile)
		if err != nil {
			return "", fmt.Errorf("failed to copy '%s' from zip to %s: %w", seleneZipFile.Name, seleneBinaryPath, err)
		}
		if err := os.Chmod(seleneBinaryPath, 0755); err != nil {
			return "", fmt.Errorf("failed to make %s executable: %w", seleneBinaryPath, err)
		}

		fmt.Printf("Selene %s (%s variant) downloaded and installed to %s\n", version, variant, seleneBinaryPath)
	} else if statErr != nil && !os.IsNotExist(statErr) {
		return "", fmt.Errorf("failed to stat selene binary at %s: %w", seleneBinaryPath, statErr)
	} else {
		fmt.Printf("Using existing Selene binary at %s for version '%s'.\n", seleneBinaryPath, version)
	}
	return seleneBinaryPath, nil
}

func decodeJSONBody(resp *http.Response, target interface{}) error {
	lr := io.LimitReader(resp.Body, 10<<20)
	return json.NewDecoder(lr).Decode(target)
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
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	if err = cmd.Start(); err != nil {
		return fmt.Errorf("failed to start Selene: %w", err)
	}

	var seleneOutput []string
	var annotations []Annotation

	scanner := bufio.NewScanner(stdoutPipe)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "{") || !strings.HasSuffix(line, "}") {
			fmt.Println(line)
			seleneOutput = append(seleneOutput, line)
			continue
		}

		fmt.Println(line)
		seleneOutput = append(seleneOutput, line)

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
				level = "failure"
			} else if levelString == "warning" {
				level = "warning"
			}

			title := fmt.Sprintf("Selene %s (%s)", strings.Title(finding.Severity), ruleName)

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
	if scanErr := scanner.Err(); scanErr != nil {
		fmt.Fprintf(os.Stderr, "Error reading Selene stdout: %v\n", scanErr)
	}

	stderrBytes, _ := io.ReadAll(stderrPipe)
	if len(stderrBytes) > 0 {
		fmt.Fprintf(os.Stderr, "Selene stderr:\n%s\n", string(stderrBytes))
	}

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
				ann.Level, ann.File, ann.Line, ann.Title, ann.Message)
		}
	}

	// Check for any "failure" level annotations (includes parse errors)
	// This must take precedence over exit code logic if critical errors are found.
	hasFailureAnnotations := false
	for _, ann := range annotations {
		if ann.Level == "failure" {
			hasFailureAnnotations = true
			break
		}
	}

	if hasFailureAnnotations {
		fmt.Fprintf(os.Stderr, "Selene reported critical errors (e.g., parse errors) via annotations. Action will fail.\n")
		// If seleneCmdErr is nil (e.g. Selene exited 0 or 1 despite parse errors),
		// create a new error. Otherwise, wrap the existing one.
		if seleneCmdErr != nil {
			return fmt.Errorf("Selene reported critical errors (see annotations). Selene error: %w", seleneCmdErr)
		}
		return fmt.Errorf("Selene reported critical errors (see annotations)")
	}

	// Original decision logic based on seleneExitCode, now only if no "failure" annotations were found:
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
	} else { // seleneExitCode > 1
		fmt.Printf("Selene exited with a critical error code: %d.\n", seleneExitCode)
		fmt.Fprintf(os.Stderr, "Action will fail.\n")
		return fmt.Errorf("Selene exited with a critical error code: %d. Selene error: %w", seleneExitCode, seleneCmdErr)
	}
}
