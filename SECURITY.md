# Security Policy

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| 1.x     | :white_check_mark: |
| < 1.0   | :x:                |

## Reporting a Vulnerability

If you discover a security vulnerability in codebak, please report it responsibly:

1. **Do NOT open a public issue** for security vulnerabilities
2. Email the maintainer directly at the address in the git commit history
3. Include:
   - Description of the vulnerability
   - Steps to reproduce
   - Potential impact
   - Any suggested fixes (optional)

## Response Timeline

- **Initial response**: Within 48 hours
- **Status update**: Within 7 days
- **Fix timeline**: Depends on severity
  - Critical: 24-48 hours
  - High: 7 days
  - Medium: 30 days
  - Low: Next release

## Security Measures

codebak implements the following security practices:

### Encryption
- Sensitive paths (SSH keys, AWS credentials, etc.) are backed up using [restic](https://restic.net/)
- AES-256 encryption at rest
- Repository password stored in macOS Keychain (not plaintext)

### Code Security
- No hardcoded credentials
- CI pipeline includes:
  - `govulncheck` for dependency vulnerabilities
  - `gosec` for Go security issues
  - `trufflehog` for secret detection

### Safe Defaults
- Backups are readable only by the owner (0755 directories, 0644 files)
- Recovery operations require explicit confirmation
- ZipSlip protection on archive extraction

## Scope

The following are **in scope** for security reports:
- Command injection vulnerabilities
- Path traversal attacks
- Credential exposure
- Encryption weaknesses
- Privilege escalation

The following are **out of scope**:
- Denial of service (local CLI tool)
- Social engineering
- Physical access attacks
