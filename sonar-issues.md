## CRITICAL

- **CODE_SMELL** `internal/scan/arp_linux.go`:22 — Refactor this method to reduce its Cognitive Complexity from 21 to the 15 allowed. _(go:S3776)_
- **CODE_SMELL** `internal/scan/icmp.go`:131 — Refactor this method to reduce its Cognitive Complexity from 41 to the 15 allowed. _(go:S3776)_
- **CODE_SMELL** `internal/scan/scan.go`:53 — Refactor this method to reduce its Cognitive Complexity from 19 to the 15 allowed. _(go:S3776)_
- **CODE_SMELL** `internal/scan/store.go`:16 — Refactor this method to reduce its Cognitive Complexity from 40 to the 15 allowed. _(go:S3776)_
- **CODE_SMELL** `internal/web/handlers.go`:142 — Define a constant instead of duplicating this literal "Content-Type" 3 times. _(go:S1192)_
- **CODE_SMELL** `internal/web/handlers.go`:182 — Define a constant instead of duplicating this literal "/login" 5 times. _(go:S1192)_
- **CODE_SMELL** `internal/web/handlers.go`:185 — Define a constant instead of duplicating this literal "Set up admin" 4 times. _(go:S1192)_
- **CODE_SMELL** `internal/web/handlers.go`:232 — Define a constant instead of duplicating this literal "Sign in" 3 times. _(go:S1192)_
- **CODE_SMELL** `internal/web/handlers.go`:253 — Define a constant instead of duplicating this literal "db error" 19 times. _(go:S1192)_
- **CODE_SMELL** `internal/web/handlers.go`:366 — Define a constant instead of duplicating this literal "scan error" 7 times. _(go:S1192)_
- **CODE_SMELL** `internal/web/handlers.go`:473 — Define a constant instead of duplicating this literal "/subnets" 3 times. _(go:S1192)_
- **CODE_SMELL** `internal/web/handlers.go`:506 — Define a constant instead of duplicating this literal "/subnets/%d" 3 times. _(go:S1192)_
- **CODE_SMELL** `internal/web/static/app.js`:16 — Unexpected var, use let or const instead. _(javascript:S3504)_
- **CODE_SMELL** `internal/web/static/app.js`:19 — Unexpected var, use let or const instead. _(javascript:S3504)_
- **CODE_SMELL** `internal/web/static/app.js`:26 — Unexpected var, use let or const instead. _(javascript:S3504)_
- **CODE_SMELL** `internal/web/static/app.js`:29 — Unexpected var, use let or const instead. _(javascript:S3504)_
- **CODE_SMELL** `internal/web/static/app.js`:30 — Unexpected var, use let or const instead. _(javascript:S3504)_
- **CODE_SMELL** `internal/web/static/app.js`:31 — Unexpected var, use let or const instead. _(javascript:S3504)_
- **CODE_SMELL** `internal/web/static/app.js`:32 — Unexpected var, use let or const instead. _(javascript:S3504)_
- **CODE_SMELL** `internal/web/static/app.js`:33 — Unexpected var, use let or const instead. _(javascript:S3504)_
- **CODE_SMELL** `internal/web/static/app.js`:43 — Unexpected var, use let or const instead. _(javascript:S3504)_
- **CODE_SMELL** `internal/web/static/app.js`:59 — Unexpected var, use let or const instead. _(javascript:S3504)_
- **CODE_SMELL** `internal/web/static/app.js`:61 — Unexpected var, use let or const instead. _(javascript:S3504)_
- **CODE_SMELL** `internal/web/static/app.js`:63 — Unexpected var, use let or const instead. _(javascript:S3504)_
- **CODE_SMELL** `internal/web/static/app.js`:68 — Unexpected var, use let or const instead. _(javascript:S3504)_
- **CODE_SMELL** `internal/web/static/app.js`:78 — Unexpected var, use let or const instead. _(javascript:S3504)_
- **CODE_SMELL** `internal/web/static/app.js`:79 — Unexpected var, use let or const instead. _(javascript:S3504)_
- **CODE_SMELL** `internal/web/static/app.js`:82 — Unexpected var, use let or const instead. _(javascript:S3504)_
- **CODE_SMELL** `internal/web/static/app.js`:83 — Unexpected var, use let or const instead. _(javascript:S3504)_
- **CODE_SMELL** `internal/web/static/app.js`:87 — Unexpected var, use let or const instead. _(javascript:S3504)_
- **CODE_SMELL** `internal/web/static/app.js`:88 — Unexpected var, use let or const instead. _(javascript:S3504)_
## MAJOR

- **CODE_SMELL** `internal/web/router.go`:63 — This function has 8 parameters, which is greater than the 7 authorized. _(go:S107)_
- **CODE_SMELL** `internal/web/static/app.js`:54 — Prefer using an optional chain expression instead, as it's more concise and easier to read. _(javascript:S6582)_
- **CODE_SMELL** `internal/web/static/app.js`:87 — Prefer `.dataset` over `getAttribute(…)`. _(javascript:S7761)_
- **CODE_SMELL** `internal/web/static/app.js`:100 — Prefer `.dataset` over `getAttribute(…)`. _(javascript:S7761)_
## MINOR

- **CODE_SMELL** `internal/web/static/app.js`:88 — Use `.includes()`, rather than `.indexOf()`, when checking for existence. _(javascript:S7765)_
