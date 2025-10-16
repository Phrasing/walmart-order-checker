# Security Documentation

## Overview

This document outlines the security measures implemented in the Walmart Order Checker application.

## Recent Security Improvements (2025-10-15)

All 10 vulnerabilities from the security audit have been addressed, plus additional improvements:

### ✅ Bonus: Token Storage Optimization (COMPLETED)

**Single Source of Truth for OAuth Tokens**
- OAuth tokens NO LONGER stored in session cookies
- Only email address stored in session (minimal data)
- Tokens loaded from encrypted database on each request
- Eliminates token duplication and synchronization issues
- Reduces cookie size from ~800 bytes to ~150 bytes
- Improved security: tokens only exist in AES-256-GCM encrypted storage

### ✅ Phase 1: Critical Fixes (COMPLETED)

1. **Secure Encryption Key Generation** - `internal/security/keys.go`
   - Proper error checking on cryptographic RNG
   - Base64 encoding for binary key data
   - Validation of key lengths (32 bytes for AES-256)
   - Production enforcement: keys MUST be set via environment variables

2. **Secure Session Cookies** - `internal/auth/oauth.go`
   - `Secure` flag: Auto-detects production/HTTPS environments
   - `SameSite=Strict`: Prevents CSRF attacks
   - `HttpOnly=true`: Prevents XSS cookie theft
   - Support for reverse proxy headers (`X-Forwarded-Proto`)

3. **Database File Permissions** - `internal/security/permissions.go`
   - Automatic permission checks on startup
   - Enforces `600` (owner-only) for `tokens.db` and WAL files
   - Enforces `700` (owner-only) for `.data/` directory
   - Auto-fixes insecure permissions with warnings

4. **WebSocket Origin Validation** - `internal/api/websocket.go`
   - Strict origin checking against allowlist
   - Configurable via `ALLOWED_WS_ORIGINS` environment variable
   - Logs rejected connections for security monitoring

### ✅ Phase 2: High Priority (COMPLETED)

5. **Rate Limiting** - `internal/api/ratelimit.go`
   - Global: 100 requests/minute per IP
   - Auth endpoints: 5 requests/minute per IP (stricter)
   - Automatic cleanup of inactive visitors
   - `Retry-After` headers on 429 responses

### ✅ Phase 3: Medium Priority (COMPLETED)

6. **Security Headers** - `internal/api/middleware.go`
   - `X-Frame-Options: DENY` - Prevents clickjacking
   - `X-Content-Type-Options: nosniff` - Prevents MIME sniffing
   - `X-XSS-Protection: 1; mode=block` - XSS protection
   - `Referrer-Policy: strict-origin-when-cross-origin` - Privacy
   - `Content-Security-Policy` - XSS and injection prevention
   - `Strict-Transport-Security` (production only) - HTTPS enforcement

7. **CORS Configuration** - `cmd/web/main.go`
   - No wildcards - explicit origins only
   - Development: localhost:3000, localhost:5173
   - Production: Configurable via `FRONTEND_URL`

## Environment Variables

### Required in Production

```bash
# MUST be set in production or server will refuse to start
ENVIRONMENT=production
SESSION_KEY=<base64-encoded-32-bytes>
ENCRYPTION_KEY=<base64-encoded-32-bytes>
```

### Generate Secure Keys

```bash
# Session key
openssl rand -base64 32

# Encryption key
openssl rand -base64 32
```

### Optional

```bash
# Frontend URL for CORS (production)
FRONTEND_URL=https://your-domain.com

# WebSocket allowed origins (comma-separated)
ALLOWED_WS_ORIGINS=https://your-domain.com,https://www.your-domain.com
```

## Cryptography

- **Algorithm**: AES-256-GCM (authenticated encryption)
- **Key Size**: 256 bits (32 bytes)
- **Nonce**: 96 bits, randomly generated per encryption
- **Authentication**: Built-in via GCM mode

## Session Management

- **Storage**: HTTP-only cookies (not accessible to JavaScript)
- **Lifetime**: 7 days (604800 seconds)
- **Security**: Signed with HMAC-SHA256
- **SameSite**: Strict mode (CSRF protection)
- **Secure**: Auto-enabled in production/HTTPS

## OAuth 2.0 Security

- **State Parameter**: Cryptographically random, validated on callback
- **Token Storage**: AES-256-GCM encrypted in SQLite
- **Token Refresh**: Automatic refresh before expiration
- **Scopes**: Gmail readonly only (principle of least privilege)

## Rate Limiting

| Endpoint Group | Limit | Burst | Purpose |
|---------------|-------|-------|---------|
| Global | 100/min | 10 | General DoS protection |
| Auth endpoints | 5/min | 2 | Prevent brute force |

## File Permissions

| Path | Permission | Purpose |
|------|-----------|---------|
| `.data/` | 0700 | Directory containing secrets |
| `.data/tokens.db` | 0600 | OAuth tokens database |
| `.data/*.db-wal` | 0600 | SQLite write-ahead log |

## Security Checklist for Production

### Pre-Deployment

- [ ] Set `ENVIRONMENT=production` in `.env`
- [ ] Generate and set `SESSION_KEY` (32 bytes, base64)
- [ ] Generate and set `ENCRYPTION_KEY` (32 bytes, base64)
- [ ] Set `FRONTEND_URL` to production domain
- [ ] Set `REDIRECT_URL` to production OAuth callback
- [ ] Update `ALLOWED_WS_ORIGINS` for production domain
- [ ] Verify SSL/TLS certificate is valid
- [ ] Enable reverse proxy (nginx/Caddy) with HTTPS

### Post-Deployment

- [ ] Verify `Secure` flag on cookies (check browser dev tools)
- [ ] Verify security headers (use https://securityheaders.com/)
- [ ] Test rate limiting (attempt >5 logins/minute)
- [ ] Verify WebSocket only accepts allowed origins
- [ ] Check file permissions: `ls -la .data/`
- [ ] Review server logs for security warnings
- [ ] Test OAuth flow end-to-end

## Monitoring

### Security Logs to Watch

```bash
# Rate limit exceeded
grep "SECURITY: Rate limit exceeded" logs.txt

# WebSocket rejections
grep "WebSocket: Rejected connection" logs.txt

# Permission warnings
grep "WARNING.*permissions" logs.txt

# Key generation warnings (should NOT appear in production)
grep "WARNING.*key.*generating temporary" logs.txt
```

## Threat Model

### Protected Against

✅ Man-in-the-Middle attacks (HTTPS + Secure cookies)
✅ Cross-Site Request Forgery (SameSite=Strict + State parameter)
✅ Cross-Site Scripting (HttpOnly cookies + CSP headers)
✅ Clickjacking (X-Frame-Options: DENY)
✅ Brute force attacks (Rate limiting on auth endpoints)
✅ Token theft from filesystem (0600 permissions + encryption)
✅ WebSocket hijacking (Origin validation)
✅ DoS attacks (Global rate limiting)

### NOT Protected Against

⚠️ Client-side attacks (malware on user's machine)
⚠️ Compromised Google account (OAuth security relies on Google)
⚠️ Physical access to server (filesystem encryption not implemented)
⚠️ Database injection (using prepared statements, but not ORM)

## Incident Response

If a security incident is suspected:

1. **Revoke OAuth tokens**: Delete `.data/tokens.db`
2. **Rotate session keys**: Generate new `SESSION_KEY`
3. **Check logs**: Review for suspicious activity
4. **Update dependencies**: `go get -u ./...`
5. **Restart server**: New keys invalidate all sessions

## Compliance

- **OWASP Top 10**: Addressed (see audit report)
- **OAuth 2.0 RFC 6749**: Compliant
- **GDPR**: User emails stored encrypted, can be deleted
- **SOC 2**: Authentication, authorization, encryption implemented

## Security Audit History

| Date | Auditor | Critical | High | Medium | Status |
|------|---------|----------|------|--------|--------|
| 2025-10-15 | Internal | 3 | 4 | 3 | ✅ All Fixed |

## Contact

For security concerns, please create an issue at:
https://github.com/your-org/walmart-order-checker/issues

## References

- [OWASP Secure Coding Practices](https://owasp.org/www-project-secure-coding-practices-quick-reference-guide/)
- [OAuth 2.0 RFC 6749](https://tools.ietf.org/html/rfc6749)
- [AES-GCM Security](https://nvlpubs.nist.gov/nistpubs/Legacy/SP/nistspecialpublication800-38d.pdf)
