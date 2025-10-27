# Security Policy

## Supported Versions

We release patches for security vulnerabilities. Currently supported versions:

| Version | Supported          |
| ------- | ------------------ |
| 0.x.x   | :white_check_mark: |

## Reporting a Vulnerability

We take the security of LangGraph-Go seriously. If you believe you have found a security vulnerability, please report it to us as described below.

**Please do not report security vulnerabilities through public GitHub issues.**

Instead, please send an email to the project maintainers with the following information:

- Type of issue (e.g. buffer overflow, SQL injection, cross-site scripting, etc.)
- Full paths of source file(s) related to the manifestation of the issue
- The location of the affected source code (tag/branch/commit or direct URL)
- Any special configuration required to reproduce the issue
- Step-by-step instructions to reproduce the issue
- Proof-of-concept or exploit code (if possible)
- Impact of the issue, including how an attacker might exploit the issue

We will acknowledge receipt of your vulnerability report and send you regular updates about our progress.

## Security Best Practices

When using LangGraph-Go in production:

### API Keys and Credentials

- **Never hardcode API keys** - Always use environment variables or secure secrets management
- **Rotate API keys regularly** - Implement a key rotation policy
- **Use separate keys for different environments** - Development, staging, and production should have different credentials

### State and Data Security

- **Encrypt sensitive state data** - When persisting state to MySQL or other stores, encrypt sensitive fields
- **Validate state inputs** - Always validate and sanitize data before storing in state
- **Use secure connections** - Always use TLS for MySQL connections in production

### LLM Integration Security

- **Validate LLM outputs** - Don't trust LLM responses for security-critical decisions
- **Implement rate limiting** - Protect against abuse of LLM APIs
- **Monitor token usage** - Track and alert on unusual usage patterns
- **Sanitize prompts** - Be cautious of prompt injection attacks

### Example: Secure Configuration

```go
// Good: API keys from environment
apiKey := os.Getenv("OPENAI_API_KEY")
if apiKey == "" {
    log.Fatal("OPENAI_API_KEY not set")
}

// Good: TLS-enabled MySQL connection
mysqlDSN := fmt.Sprintf("%s:%s@tcp(%s)/%s?tls=true",
    os.Getenv("DB_USER"),
    os.Getenv("DB_PASS"),
    os.Getenv("DB_HOST"),
    os.Getenv("DB_NAME"),
)

// Bad: Hardcoded credentials (never do this!)
// apiKey := "sk-proj-abc123..."
```

## Known Security Considerations

### MySQL Store

- The MySQL store implementation uses parameterized queries to prevent SQL injection
- State data is serialized to JSON before storage - ensure sensitive data is encrypted before serialization
- Connection strings should use TLS in production environments

### LLM Providers

- API keys are passed to official SDK clients (OpenAI, Anthropic, Google)
- These SDKs handle secure communication with provider APIs
- Always use the latest SDK versions to get security patches

### Tool Execution

- Tools can execute arbitrary code - only register trusted tool implementations
- Validate tool inputs before execution
- Consider sandboxing tool execution in production environments

## Security Updates

Security updates will be announced through:
- GitHub Security Advisories
- Release notes
- Project README

Subscribe to repository notifications to stay informed about security updates.
