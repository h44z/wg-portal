This section describes the security features available to administrators for hardening WireGuard Portal and protecting its data.

## Database Encryption

WireGuard Portal supports multiple database backends. To reduce the risk of data exposure, sensitive information stored in the database can be encrypted.
To enable encryption, set the [`encryption_passphrase`](../configuration/overview.md#database) in the database configuration section.

> :warning: Important: Once encryption is enabled, it cannot be disabled, and the passphrase cannot be changed! 
> Only new or updated records will be encrypted; existing data remains in plaintext until it’s next modified.

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