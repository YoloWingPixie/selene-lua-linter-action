# Selene Lua Linter Action

Run Selene from a prebuilt container and report findings as GitHub annotations.
The action image includes Selene 0.30.1 and its light variant. Consumer jobs do
not build the action or download Selene.

## Usage

```yaml
- name: Lint Lua
  uses: YoloWingPixie/selene-lua-linter-action@v1
  with:
    config-path: selene.toml
    lint-path: src
    fail-on-warnings: "false"
```

## Inputs

| Input | Required | Default | Purpose |
|---|---:|---|---|
| `config-path` | Yes | | Selene configuration relative to the working directory. |
| `working-directory` | No | `.` | Directory in which Selene runs. |
| `lint-path` | No | `.` | File or directory passed to Selene. |
| `selene-args` | No | | Additional Selene arguments. |
| `fail-on-warnings` | No | `false` | Fail when Selene reports warnings. |
| `report-as-annotations` | No | `true` | Emit GitHub workflow annotations. |
| `selene-variant` | No | `selene` | Run `selene` or `selene-light`. |

`selene-version` and `selene-repo` remain accepted for workflow compatibility,
but the prebuilt image ignores them.

## Releases

The `dev` branch publishes the mutable `dev` image tag and tests that image through the action metadata.

1. Set the exact image tag in `action.yml`.
2. Commit the release change.
3. Create and push the matching semantic-version tag.
4. Make the GHCR package public after its first publication.
5. Move the major action tag after the image publication succeeds.

The release workflow publishes only the exact image tag referenced by
`action.yml`.
