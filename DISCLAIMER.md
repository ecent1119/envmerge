# DISCLAIMER

## envmerge — Environment Resolution Inspector

### Scope of Tool

This tool is designed for **local development environments only**. It inspects environment variable configurations to help developers understand resolution order and detect potential conflicts.

### What This Tool Does

- Reads `.env` files and Docker Compose configurations
- Analyzes environment variable precedence
- Reports final resolved values
- Optionally outputs an effective env file

### What This Tool Does NOT Do

- Modify your existing environment files (unless explicitly requested via `--output`)
- Execute any Docker or Compose commands
- Make network requests
- Access production systems
- Provide security scanning or vulnerability assessment
- Guarantee correctness of your environment configuration

### Liability

This software is provided **"as is"** without warranty of any kind, express or implied.

The authors and distributors:
- Make no guarantees about accuracy of resolution analysis
- Accept no responsibility for misconfigurations in your environment
- Are not liable for any damages arising from use of this tool
- Do not warrant fitness for any particular purpose

### Recommended Use

- Development and testing environments only
- As a diagnostic aid, not a source of truth
- In conjunction with manual verification
- Before deploying to production, verify configurations independently

### Not For

- Production deployment decisions
- Security auditing
- Compliance verification
- Automated CI/CD without human review
