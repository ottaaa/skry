#!/usr/bin/env bash
# Seed a tiny git repo with a mix of committed, modified, added, and
# untracked files so `make dev` has something interesting to render.
#
# Idempotent: re-running wipes and re-creates. Never touches anything
# outside its own directory.

set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$HERE"

# Wipe any previous state. This directory is listed in .gitignore so we
# only ever own what's under $HERE.
rm -rf .git .gitignore README.md src docs notes.txt

mkdir -p src docs

cat >README.md <<'EOF'
# sample-repo

Fixture project used by `make dev`. See `seed.sh` for how this is
constructed. Feel free to poke at it — re-running `make dev` will
re-seed from scratch.
EOF

cat >src/main.go <<'EOF'
package main

import "fmt"

func main() {
	fmt.Println("hello, skry")
}
EOF

cat >src/util.go <<'EOF'
package main

func add(a, b int) int { return a + b }
EOF

cat >docs/intro.md <<'EOF'
# Intro

This file is committed; modifying it should show up as `M` in the tree.
EOF

git init -q -b main .

# seed.sh is part of the parent repo, not this fixture.
cat >.gitignore <<'EOF'
seed.sh
EOF

git -c user.email=dev@skry.local -c user.name=skry-dev add .gitignore README.md src docs
git -c user.email=dev@skry.local -c user.name=skry-dev commit -q -m "initial commit"

# Modify a committed file so the diff view has something to show.
cat >>docs/intro.md <<'EOF'

## Changes

Added this section to generate a working-tree diff.
EOF

# Add a staged-but-uncommitted file.
cat >src/new_feature.go <<'EOF'
package main

// Added in working tree but not committed.
func subtract(a, b int) int { return a - b }
EOF

# And one fully untracked file.
cat >notes.txt <<'EOF'
Untracked scratch notes.
EOF

echo "sample-repo seeded at $HERE"
