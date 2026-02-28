For all external authentication providers (LDAP, OIDC, OAuth2), WireGuard Portal can automatically create a local user record upon the user's first successful login.
This behavior is controlled by the `registration_enabled` setting in each authentication provider's configuration.

User information from external authentication sources is merged into the corresponding local WireGuard Portal user record whenever the user logs in.
Additionally, WireGuard Portal supports periodic synchronization of user data from an LDAP directory.

To prevent overwriting local changes, WireGuard Portal allows you to set a per-user flag that disables synchronization of external attributes.
When this flag is set, the user in WireGuard Portal will not be updated automatically during log-ins or LDAP synchronization.

### LDAP Synchronization

WireGuard Portal lets you hook up any LDAP server such as Active Directory or OpenLDAP for both authentication and user sync.
You can even register multiple LDAP servers side-by-side. Details on the log-in process can be found in the [LDAP Authentication](./authentication.md#ldap-authentication) section.

If you enable LDAP synchronization, all users within the LDAP directory will be created automatically in the WireGuard Portal database if they do not exist.
If a user is disabled or deleted in LDAP, the user will be disabled in WireGuard Portal as well.
The synchronization process can be fine-tuned by multiple parameters, which are described below.

#### Synchronization Parameters

To enable the LDAP sycnhronization this feature, set the `sync_interval` property in the LDAP provider configuration to a value greater than "0".
The value is a string representing a duration, such as "15m" for 15 minutes or "1h" for 1 hour (check the [exact format definition](https://pkg.go.dev/time#ParseDuration) for details).
The synchronization process will run in the background and synchronize users from LDAP to the database at the specified interval.
Also make sure that the `sync_filter` property is a well-formed LDAP filter, or synchronization will fail.

##### Limiting Synchronization to Specific Users

Use the `sync_filter` property in your LDAP provider block to restrict which users get synchronized.
It accepts any valid LDAP search filter, only entries matching that filter will be pulled into the portal's database.

For example, to import only users with a `mail` attribute:
```yaml
auth:
  ldap:
    - id: ldap
      # ... other settings
      sync_filter: (mail=*)
```

##### Disable Missing Users

If you set the `disable_missing` property to `true`, any user that is not found in LDAP during synchronization will be disabled in WireGuard Portal.
All peers associated with that user will also be disabled.

If you want a user and its peers to be automatically re-enabled once they are found in LDAP again, set the `auto_re_enable` property to `true`.
This will only re-enable the user if they were disabled by the synchronization process. Manually disabled users will not be re-enabled.