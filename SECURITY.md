# Security Policy

## Supported Versions

| Version | Supported |
|---------|-----------|
| latest  | Yes       |

## Reporting a Vulnerability

If you discover a security vulnerability, please report it responsibly:

1. **Do not** open a public GitHub issue.
2. Email the maintainer at **pgarciaq@redhat.com** with:
   - A description of the vulnerability.
   - Steps to reproduce.
   - Potential impact assessment.
3. You will receive an acknowledgment within 48 hours.
4. A fix will be developed privately and released as a patch version.

## Security Considerations

- The `x-rh-identity` header contains base64-encoded identity data. By default, the provider validates that `KOKU_API_URL` uses HTTPS unless `KOKU_ALLOW_INSECURE=true` is explicitly set or the target is localhost.
- NATS communication should be secured with TLS in production deployments.
- The SQLite database file should have appropriate filesystem permissions (0600).
- Container images are built on UBI (Universal Base Image) minimal for a reduced attack surface.
