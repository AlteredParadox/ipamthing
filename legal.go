// The repo-root legal texts are embedded here so every distributed artifact
// carries its own license and the third-party notices for everything
// statically linked into it. The web server serves them unauthenticated at
// /license and /third-party-licenses (see internal/web/legal.go), and
// scripts/gen-third-party-licenses.sh regenerates the notices.

package main

import _ "embed"

// license is the project's own license (GNU AGPL-3.0-or-later).
//
//go:embed LICENSE
var license []byte

// thirdPartyLicenses reproduces the license and copyright notices of all Go
// modules linked into the binary. Regenerate with `make notices` after any
// dependency change; `make notices-check` (run by the release workflow)
// fails if the committed copy is stale.
//
//go:embed THIRD_PARTY_LICENSES.md
var thirdPartyLicenses []byte
