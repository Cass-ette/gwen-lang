# Gwen Release Checklist

This checklist is for patch releases on the current Go mainline.

## Release Line Rules

- Python reference implementation remains `v0.1.0` unless explicitly releasing the Python line.
- Go mainline releases use the current `v0.2.x` line.
- VSCode extension version should match the Go release version when shipping a new extension package.

## Before Editing

- Confirm the current branch and target branch.
- Confirm whether the change is docs-only or behavior-changing.
- For behavior changes, identify the smallest relevant checker/runtime/compiler tests before editing.

## Version Updates

For a Go mainline patch release:

- update `cmd/gwen/main.go` if the CLI version changes
- update `vscode-extension/package.json` if the extension version changes
- update `vscode-extension/README.md` install commands if the VSIX name changes
- regenerate the VSIX with `vsce package` from `vscode-extension/`

Do not change `gwen/__init__.py` or Python CLI version tests unless releasing the Python reference line.

## Validation

Run focused validation first:

```bash
go test ./cmd/gwen
go run ./cmd/gwen --version
python -m pytest tests/test_cli.py
```

If core language behavior, checker/runtime/compiler internals, or examples changed, also run:

```bash
go test ./...
```

If the VSCode extension package changed, verify package contents:

```bash
cd vscode-extension
vsce ls
```

## GitHub Release Flow

```bash
git status --short
git log --oneline -5
git tag --list 'vX.Y.Z'
git add <changed files>
git commit -m "release: prepare Gwen vX.Y.Z"
git push origin main
git tag -a vX.Y.Z -m "Gwen vX.Y.Z"
git push origin vX.Y.Z
gh release create vX.Y.Z vscode-extension/gwen-lang-X.Y.Z.vsix --title "Gwen vX.Y.Z" --notes-file <notes-file>
gh release view vX.Y.Z --json url,assets,tagName
```

Use a release notes file or heredoc for non-trivial notes.

## Final Verification

Before announcing completion, confirm:

- working tree is clean
- latest commit is on `main`
- tag points at the intended commit
- release exists
- expected assets are attached
- validation commands and results are recorded in the final response
