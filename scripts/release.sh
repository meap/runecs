#!/usr/bin/env bash
# Cut a release: finalize CHANGELOG.md, commit to main, tag, push.
# Usage: make release V=x.y.z
set -euo pipefail

die() { echo "release: $*" >&2; exit 1; }

V="${1:-}"
[[ "$V" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || die "usage: make release V=x.y.z"
TAG="v$V"
REPO_URL="https://github.com/meap/runecs"
CHANGELOG="CHANGELOG.md"

cd "$(git rev-parse --show-toplevel)"

[[ "$(git branch --show-current)" == "main" ]] || die "not on main"
[[ -z "$(git status --porcelain)" ]] || die "working tree is not clean"
git fetch -q --tags origin main
[[ "$(git rev-parse HEAD)" == "$(git rev-parse origin/main)" ]] || die "main is not in sync with origin/main"
! git rev-parse -q --verify "refs/tags/$TAG" >/dev/null || die "tag $TAG already exists"
PREV="$(git describe --tags --abbrev=0)"

grep -qx '## \[Unreleased\]' "$CHANGELOG" || die "no '## [Unreleased]' section in $CHANGELOG"
NOTES="$(awk '/^## \[Unreleased\]$/ {p=1; next} p && /^## \[/ {exit} p' "$CHANGELOG")"
grep -q '[^[:space:]]' <<<"$NOTES" || die "'## [Unreleased]' section is empty"

# Rename [Unreleased] to the version, open a new empty [Unreleased],
# and add the compare link above the existing ones (or at the end).
TMP="$(mktemp)"
awk -v ver="$V" -v date="$(date +%Y-%m-%d)" \
    -v link="[$V]: $REPO_URL/compare/$PREV...$TAG" '
  /^## \[Unreleased\]$/ { print; print ""; print "## [" ver "] - " date; next }
  !done && /^\[[0-9]+\.[0-9]+\.[0-9]+\]: / { print link; done=1 }
  { print }
  END { if (!done) print link }
' "$CHANGELOG" >"$TMP"
mv "$TMP" "$CHANGELOG"

git --no-pager diff -- "$CHANGELOG"
read -r -p "Release $TAG (previous $PREV)? Commits to main and pushes the tag. [y/N] " ok
if [[ "$ok" != "y" ]]; then
  git checkout -- "$CHANGELOG"
  die "aborted, $CHANGELOG restored"
fi

git commit -q -m "Release $TAG" -- "$CHANGELOG"
git tag -a "$TAG" -m "$TAG"
git push --atomic origin main "$TAG"
echo "Pushed $TAG. Release workflow: $REPO_URL/actions"
