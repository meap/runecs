# How to release

## 1. Record changes during development

Add every user-facing change under `## [Unreleased]` in `CHANGELOG.md`:

```markdown
## [Unreleased]

### New
- Short description of the new feature.

### Fixed
- Short description of the fix.
```

## 2. Release

```bash
git switch main
git pull
make release V=0.11.0
```

The command shows the `CHANGELOG.md` diff and asks for confirmation. Type `y`.

That's it. GitHub Actions builds the binaries, creates the GitHub release, updates Homebrew, and pushes the Docker image.
Check progress at https://github.com/meap/runecs/actions.

## If it fails

- `working tree is not clean`: commit or stash your local changes.
- `main is not in sync with origin/main`: run `git pull`.
- `'## [Unreleased]' section is empty`: add changes to `CHANGELOG.md` first.
- `tag vX.Y.Z already exists`: pick a higher version.
