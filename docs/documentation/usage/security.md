This section describes the security features available to administrators for hardening WireGuard Portal and protecting its data.

## Database Encryption

WireGuard Portal supports multiple database backends. To reduce the risk of data exposure, sensitive information stored in the database can be encrypted.
To enable encryption, set the [`encryption_passphrase`](../configuration/overview.md#database) in the database configuration section.

> :warning: Important: Once encryption is enabled, it cannot be disabled, and the passphrase cannot be changed! 
> Only new or updated records will be encrypted; existing data remains in plaintext until it’s next modified.

## External Identity Provider Data Sanitization

When users authenticate via LDAP, OIDC, or OAuth, WireGuard Portal sanitizes the field values received from the provider before storing them. This protects against several classes of attack that a compromised or misconfigured identity provider could introduce:

- **Unsafe control characters** — Unicode control and format characters, null bytes, and invalid UTF-8 bytes are stripped from external profile fields before they reach the Vue.js UI or email templates.
- **Email header injection** — carriage return and line feed characters in email fields are rejected entirely, and email fields must parse as plain email addresses.
- **Log injection** — unsafe control and format characters are stripped from all external profile fields and from sanitization log context.
- **Denial of service via oversized fields** — field lengths are capped (e.g., 256 runes for identifiers, 254 characters for email addresses).
- **Reserved identifier collision** — reserved user identifiers such as `"all"`, `"new"`, `"id"`, and internal system user identifiers are rejected.
- **Unsafe authorization groups** — OIDC/OAuth group claims are sanitized before group-based checks; groups changed by control/format stripping or truncation are dropped rather than repaired into allowed/admin matches.

Sanitization is **enabled by default**. It can be disabled via the [`sanitize_external_user_data`](../configuration/overview.md#sanitize_external_user_data) configuration key.

> :warning: Only disable sanitization if your identity provider is fully under your control and you have confirmed that sanitization causes unacceptable data loss (for example, legitimate usernames that contain characters the sanitizer strips).

When sanitization modifies or clears a field value, a `WARN` log entry is emitted with the provider name, provider type, and field name — but never the raw or sanitized value, to avoid leaking sensitive data into logs. This makes it straightforward to detect and investigate potentially malicious or misconfigured providers.

---

## UI and API Access

WireGuard Portal provides a web UI and a REST API for user interaction. It is important to secure these interfaces to prevent unauthorized access and data breaches.

### HTTPS
It is recommended to use HTTPS for all communication with the portal to prevent eavesdropping. 

Event though, WireGuard Portal supports HTTPS out of the box, it is recommended to use a reverse proxy like Nginx or Traefik to handle SSL termination and other security features.
A detailed explanation is available in the [Reverse Proxy](../getting-started/reverse-proxy.md) section.

### Secure Authentication
To prevent unauthorized access, WireGuard Portal supports integrating with secure authentication providers such as LDAP, OAuth2, or Passkeys, see [Authentication](./authentication.md) for more details.
When possible, use centralized authentication and enforce multi-factor authentication (MFA) at the provider level for enhanced account security.
For local accounts, administrators should enforce strong password requirements.
