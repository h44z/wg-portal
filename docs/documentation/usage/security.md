This section describes the security features available to administrators for hardening WireGuard Portal and protecting its data.

## Authentication

WireGuard Portal supports multiple authentication methods, including:

- Local user accounts
- LDAP authentication
- OAuth and OIDC authentication
- Passkey authentication (WebAuthn)

Users can have two roles which limit their permissions in WireGuard Portal:

- **User**: Can manage their own account and peers.
- **Admin**: Can manage all users and peers, including the ability to manage WireGuard interfaces.

### Password Security

WireGuard Portal supports username and password authentication for both local and LDAP-backed accounts.
Local users are stored in the database, while LDAP users are authenticated against an external LDAP server.

On initial startup, WireGuard Portal automatically creates a local admin account with the password `wgportal-default`.
> :warning: This password must be changed immediately after the first login.

The minimum password length for all local users can be configured in the [`auth`](../configuration/overview.md#auth) 
section of the configuration file. The default value is **16** characters, see [`min_password_length`](../configuration/overview.md#min_password_length).
The minimum password length is also enforced for the default admin user.


### Passkey (WebAuthn) Authentication

Besides the standard authentication mechanisms, WireGuard Portal supports Passkey authentication.
This feature is enabled by default and can be configured in the [`webauthn`](../configuration/overview.md#webauthn-passkeys) section of the configuration file.

Users can register multiple Passkeys to their account. These Passkeys can be used to log in to the web UI as long as the user is not locked.
> :warning: Passkey authentication does not disable password authentication. The password can still be used to log in (e.g., as a fallback).

To register a Passkey, open the settings page *(1)* in the web UI and click on the "Register Passkey" *(2)* button.

![Passkey UI](../../assets/images/passkey_setup.png)


### OAuth and OIDC Authentication

WireGuard Portal supports OAuth and OIDC authentication. You can use any OAuth or OIDC provider that supports the authorization code flow, 
such as Google, GitHub, or Keycloak.

For OAuth or OIDC to work, you need to configure the [`external_url`](../configuration/overview.md#external_url) property in the [`web`](../configuration/overview.md#web) section of the configuration file.
If you are planning to expose the portal to the internet, make sure that the `external_url` is configured to use HTTPS.

To add OIDC or OAuth authentication to WireGuard Portal, create a Client-ID and Client-Secret in your OAuth provider and 
configure a new authentication provider in the [`auth`](../configuration/overview.md#auth) section of the configuration file.
Make sure that each configured provider has a unique `provider_name` property set. Samples can be seen [here](../configuration/examples.md).

#### Limiting Login to Specific Domains

You can limit the login to specific domains by setting the `allowed_domains` property for OAuth or OIDC providers.
This property is a comma-separated list of domains that are allowed to log in. The user's email address is checked against this list.
For example, if you want to allow only users with an email address ending in `outlook.com` to log in, set the property as follows:

```yaml
auth:
  oidc:
    - provider_name: "oidc1"
      # ... other settings
      allowed_domains:
        - "outlook.com"
```

#### Limit Login to Existing Users

You can limit the login to existing users only by setting the `registration_enabled` property to `false` for OAuth or OIDC providers.
If registration is enabled, new users will be created in the database when they log in for the first time.

#### Admin Mapping

You can map users to admin roles based on their attributes in the OAuth or OIDC provider. To do this, set the `admin_mapping` property for the provider.
Administrative access can either be mapped by a specific attribute or by group membership.

**Attribute specific mapping** can be achieved by setting the `admin_value_regex` and the `is_admin` property.
The `admin_value_regex` property is a regular expression that is matched against the value of the `is_admin` attribute.
The user is granted admin access if the regex matches the attribute value.

Example:
```yaml
auth:
  oidc:
    - provider_name: "oidc1"
      # ... other settings
      field_map:
        is_admin: "wg_admin_prop"
      admin_mapping:
        admin_value_regex: "^true$"
```
The example above will grant admin access to users with the `wg_admin_prop` attribute set to `true`.

**Group membership mapping** can be achieved by setting the `admin_group_regex` and `user_groups` property.
The `admin_group_regex` property is a regular expression that is matched against the group names of the user.
The user is granted admin access if the regex matches any of the group names.

Example:
```yaml
auth:
  oidc:
    - provider_name: "oidc1"
      # ... other settings
      field_map:
        user_groups: "groups"
      admin_mapping:
        admin_group_regex: "^the-admin-group$"
```
The example above will grant admin access to users who are members of the `the-admin-group` group.


### LDAP Authentication

WireGuard Portal supports LDAP authentication. You can use any LDAP server that supports the LDAP protocol, such as Active Directory or OpenLDAP.
Multiple LDAP servers can be configured in the [`auth`](../configuration/overview.md#auth) section of the configuration file. 
WireGuard Portal remembers the authentication provider of the user and therefore avoids conflicts between multiple LDAP providers.

To configure LDAP authentication, create a new [`ldap`](../configuration/overview.md#ldap) authentication provider in the [`auth`](../configuration/overview.md#auth) section of the configuration file.

#### Limiting Login to Specific Users

You can limit the login to specific users by setting the `login_filter` property for LDAP provider. This filter uses the LDAP search filter syntax.
The username can be inserted into the query by placing the `{{login_identifier}}` placeholder in the filter. This placeholder will then be replaced with the username entered by the user during login.

For example, if you want to allow only users with the `objectClass` attribute set to `organizationalPerson` to log in, set the property as follows:

```yaml
auth:
  ldap:
    - provider_name: "ldap1"
      # ... other settings
      login_filter: "(&(objectClass=organizationalPerson)(uid={{login_identifier}}))"
```

The `login_filter` should always be designed to return at most one user.

#### Limit Login to Existing Users

You can limit the login to existing users only by setting the `registration_enabled` property to `false` for LDAP providers.
If registration is enabled, new users will be created in the database when they log in for the first time.

#### Admin Mapping

You can map users to admin roles based on their group membership in the LDAP server. To do this, set the `admin_group` and `memberof` property for the provider.
The `admin_group` property defines the distinguished name of the group that is allowed to log in as admin. 
All groups that are listed in the `memberof` attribute of the user will be checked against this group. If one of the groups matches, the user is granted admin access.


## UI and API Access

WireGuard Portal provides a web UI and a REST API for user interaction. It is important to secure these interfaces to prevent unauthorized access and data breaches.

### HTTPS
It is recommended to use HTTPS for all communication with the portal to prevent eavesdropping. 

Event though, WireGuard Portal supports HTTPS out of the box, it is recommended to use a reverse proxy like Nginx or Traefik to handle SSL termination and other security features.
A detailed explanation is available in the [Reverse Proxy](../getting-started/reverse-proxy.md) section.