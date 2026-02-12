# envmerge

**Free and open source** — Environment Resolution Inspector. Explain what env values actually resolve to, and why.

## The Problem

Developers don't understand env precedence across:

- `.env`
- `.env.local`
- `.env.example`
- compose `env_file`
- compose inline `environment`

This causes silent misconfigurations that waste hours of debugging.

## What It Does

- Resolves final value per variable
- Shows the complete precedence chain
- Flags conflicts and overrides
- Optionally emits a resolved `.env.effective` file
- **Include OS environment variables** in resolution chain
- **Per-service filtering** to see only one service's vars
- **Compare environments** between directories
- **Strict mode** to fail on undefined variables

## Usage

```bash
# Scan current directory
envmerge scan

# Output effective env file
envmerge scan --output .env.effective

# JSON output for scripting
envmerge scan --format json

# Markdown for documentation
envmerge scan --format markdown

# Include OS environment variables
envmerge scan --include-os-env

# Show only variables for a specific service
envmerge scan --service api

# Fail if any variables are undefined
envmerge scan --strict

# Compare two environments
envmerge scan --compare ./staging
```

## Example Output

```
DATABASE_URL
  final: postgres://prod.example.com/db
  from: docker-compose.yml (service: api)
  chain:
    → docker-compose.yml:15 = postgres://prod.example.com/db
      .env.local:3 = postgres://localhost/dev
      .env:3 = postgres://localhost/db
```

## Scope

- **Read-only** by default
- **Local development** focused
- **Observational** — does not modify your environment

## Installation

Download the appropriate binary for your platform from the GitHub releases page.

## Related Tools

- [stackgen](https://github.com/ecent1119/stackgen) — Generate local Docker Compose stacks
- [envgraph](https://github.com/ecent1119/envgraph) — Visualize env variable dependencies
- [compose-diff](https://github.com/ecent1119/compose-diff) — Semantic diff for compose files
- [devcheck](https://github.com/ecent1119/devcheck) — Local project readiness inspector

## Support This Project

**envmerge is free and open source.**

If this tool saved you time, consider sponsoring:

[![Sponsor on GitHub](https://img.shields.io/badge/Sponsor-❤️-red?logo=github)](https://github.com/sponsors/ecent1119)

Your support helps maintain and improve this tool.

## License

MIT License. See LICENSE file.
