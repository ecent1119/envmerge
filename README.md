# envmerge

See exactly how your environment variables resolve.

---

## The problem

- `.env`, `.env.local`, `.env.development`, compose inline — which one wins?
- Variables get overridden and you don't know where the final value comes from
- Debugging "wrong value" issues means grepping through 5+ files
- Team members have different local overrides causing "works on my machine"

---

## What it does

- Scans all env sources: `.env`, `.env.local`, `.env.*.local`, compose files
- Shows the **final resolved value** for each variable
- Traces the **precedence chain** — see exactly which file "won"
- Detects **overrides** and shows what was overwritten
- Outputs in text, JSON, or markdown

---

## Example output

```
$ envmerge resolve

Environment Resolution Report
=============================

DATABASE_URL = postgres://localhost/myapp
  └─ .env (base)
  └─ .env.local (override) ✓ FINAL

API_KEY = sk-dev-xxxxx
  └─ .env (base)
  └─ docker-compose.yml:services.api.environment (override) ✓ FINAL

DEBUG = true
  └─ .env.development (base) ✓ FINAL

PORT = 3000
  └─ .env (base)
  └─ .env.local (override)
  └─ .env.development.local (override) ✓ FINAL

Resolved: 24 variables from 5 sources
Overrides detected: 8
```

---

## Precedence order

From lowest to highest:

1. `.env` — base defaults
2. `.env.local` — local overrides (gitignored)
3. `.env.{environment}` — environment-specific
4. `.env.{environment}.local` — environment + local
5. `docker-compose.yml` inline environment
6. `docker-compose.override.yml` inline environment

---

## Commands

```bash
# Show resolved values
envmerge resolve

# Show full chain for specific variable
envmerge resolve --var DATABASE_URL

# Output as JSON for scripting
envmerge resolve --format json

# Check a specific directory
envmerge resolve --path ./services/api
```

---

## Use cases

- **Debugging**: "Why is my API_KEY wrong?" — see the override chain
- **Onboarding**: Show new devs what variables they need to set locally
- **CI validation**: Ensure all required variables resolve to non-placeholder values
- **Documentation**: Generate env docs with source information

---

## Scope

- Local development and testing only
- Read-only analysis of files
- No secrets management
- No cloud provider integration
- No telemetry, no network calls

---

## Get it

**$29** — one-time purchase, standalone macOS/Linux/Windows binary.

👉 [Download on Gumroad](https://ecent.gumroad.com/l/junnll)

---

## Related tools

| Tool | Purpose |
|------|---------|
| **[stackgen](https://github.com/ecent119/stackgen)** | Generate Docker Compose stacks |
| **[envgraph](https://github.com/ecent119/envgraph)** | Visualize env var dependencies |
| **[envdoc](https://github.com/ecent119/envdoc)** | Generate env documentation |
| **[compose-flatten](https://github.com/ecent119/compose-flatten)** | Merge compose files |

---

## License

MIT — this repository contains documentation and examples only.
