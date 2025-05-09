# Selene Lua Linter Action

[![CI - Selene Lua Linter Action](https://github.com/YoloWingPixie/selene-lua-linter-action/workflows/CI%20-%20Selene%20Lua%20Linter%20Action/badge.svg)](https://github.com/YoloWingPixie/selene-lua-linter-action/actions?query=workflow%3ACI+-+Selene+Lua+Linter+Action)

**TL;DR: Lint your Lua code with [Selene](https://github.com/Kampfkarren/selene) directly in your GitHub Actions workflows. Get consistent results, code annotations, and fine-grained control.**

This action lints Lua code using the Selene linter. It's designed to be highly configurable, report findings as GitHub code annotations, and run within a Docker container for consistent linting environments.

## Features

*   **Selene Powered:** Leverages the powerful Selene Lua linter.
*   **GitHub Annotations:** Reports linting issues directly as annotations in your pull requests and commit checks.
*   **Highly Configurable:** Control working directory, Selene configuration file path, specific files/directories to lint, and pass additional arguments to Selene.
*   **Version Control:** Specify the Selene version, repository, and variant (full or light) to use.
*   **Dockerized:** Ensures a consistent linting environment by running Selene inside a Docker container.
*   **Workflow Control:** Option to fail the action on warnings.

## Usage

To use this action, add the following step to your GitHub Actions workflow YAML file (e.g., `.github/workflows/lint.yml`):

```yaml
name: Lint Lua Code

on: [push, pull_request]

jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Lint Lua code with Selene
        uses: YoloWingPixie/selene-lua-linter-action@v1 # Or your desired version/commit SHA
        with:
          # Required: Path to your Selene configuration file (e.g., selene.toml or .selene.toml)
          config-path: 'selene.toml'

          # Optional: Directory to run Selene in. Defaults to repository root.
          # working-directory: '.'

          # Optional: File or directory to lint, relative to working-directory. Defaults to '.' (all files).
          # lint-path: 'src/'

          # Optional: Additional arguments for Selene CLI.
          # selene-args: '--display-style quiet'

          # Optional: Fail the action if Selene reports warnings. Defaults to 'false'.
          # fail-on-warnings: 'true'

          # Optional: Report Selene findings as GitHub annotations. Defaults to 'true'.
          # report-as-annotations: 'true'

          # Optional: Selene version to use. Defaults to 'latest'.
          # selene-version: 'v0.28.0'

          # Optional: Selene repository. Defaults to 'Kampfkarren/selene'.
          # selene-repo: 'Kampfkarren/selene'

          # Optional: Selene variant ('selene' or 'selene-light'). Defaults to 'selene'.
          # selene-variant: 'selene-light'
```

### Inputs

| Name                    | Description                                                                                                | Required | Default                 |
| ----------------------- | ---------------------------------------------------------------------------------------------------------- | -------- | ----------------------- |
| `working-directory`     | Directory where Selene will be executed, relative to repository root.                                      | `false`  | `.`                     |
| `config-path`           | Path to Selene configuration file (e.g., `selene.toml`), relative to repository root.                      | `true`   |                         |
| `lint-path`             | File or directory to lint, relative to `working-directory`.                                                | `false`  | `.`                     |
| `selene-args`           | Additional arguments to pass directly to the Selene CLI.                                                   | `false`  | `''`                    |
| `fail-on-warnings`      | If `true`, the action fails if Selene reports any warnings (exit code 1).                                  | `false`  | `false`                 |
| `report-as-annotations` | If `true`, Selene findings are reported as GitHub code annotations.                                        | `false`  | `true`                  |
| `selene-version`        | Version of Selene to use (e.g., 'v0.28.0', 'latest'). 'latest' resolves to the newest release.             | `false`  | `latest`                |
| `selene-repo`           | Repository for Selene releases (owner/repo).                                                               | `false`  | `Kampfkarren/selene`    |
| `selene-variant`        | Selene variant to download (`selene` or `selene-light`).                                                     | `false`  | `selene`                |

## Configuration

1.  **Create a Selene configuration file** (e.g., `selene.toml` or `.selene.toml`) in your repository. Refer to the [Selene documentation](https://kampfkarren.github.io/selene/usage/configuration.html) for configuration options.
2.  **Update your workflow file** (e.g., `.github/workflows/lint.yml`) to use this action, ensuring the `config-path` input points to your Selene configuration file.

## Contributing

Contributions are welcome! Please feel free to submit a pull request or open an issue.

## License

This project is licensed under the terms of the MIT license. See [LICENSE](LICENSE) for details.