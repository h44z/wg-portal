Below are some sample YAML configurations demonstrating how to override some default values.

## Basic

```yaml
core:
  admin_user: test@example.com
  admin_password: password
  admin_api_token: super-s3cr3t-api-token-or-a-UUID
  import_existing: false
  create_default_peer: true
  self_provisioning_allowed: true

web:
  site_title: My WireGuard Server
  site_company_name: My Company
  listening_address: :8080
  external_url: https://my.external-domain.com
  csrf_secret: super-s3cr3t-csrf
  session_secret: super-s3cr3t-session
  request_logging: true

advanced:
  log_level: trace
  log_pretty: true
  log_json: false
  config_storage_path: /etc/wireguard
  expiry_check_interval: 5m

database:
  debug: true
  type: sqlite
  dsn: data/sqlite.db
  encryption_passphrase: change-this-s3cr3t-encryption-passphrase

auth:
  webauthn:
    enabled: true
```

## LDAP Authentication and Synchronization

```yaml
# ... (basic configuration)

auth:
  ldap:
    # a sample LDAP provider with user sync enabled
    - id: ldap
      provider_name: Active Directory
      url: ldap://srv-ad1.company.local:389
      bind_user: ldap_wireguard@company.local
      bind_pass: super-s3cr3t-ldap
      base_dn: DC=COMPANY,DC=LOCAL
      login_filter: (&(objectClass=organizationalPerson)(mail={{login_identifier}})(!userAccountControl:1.2.840.113556.1.4.803:=2))
      sync_interval: 15m
      sync_filter: (&(objectClass=organizationalPerson)(!userAccountControl:1.2.840.113556.1.4.803:=2)(mail=*))
      disable_missing: true
      field_map:
        user_identifier: sAMAccountName
        email: mail
        firstname: givenName
        lastname: sn
        phone: telephoneNumber
        department: department
        memberof: memberOf
      admin_group: CN=WireGuardAdmins,OU=Some-OU,DC=COMPANY,DC=LOCAL
      registration_enabled: true
      log_user_info: true
```

## OpenID Connect (OIDC) Authentication

```yaml
# ... (basic configuration)

auth:
  oidc:
    # A sample Entra ID provider with environment variable substitution.
    # Only users with an @outlook.com email address are allowed to register or login.
    - id: azure
      provider_name: azure
      display_name: Login with</br>Entra ID
      registration_enabled: true
      base_url: "https://login.microsoftonline.com/${AZURE_TENANT_ID}/v2.0"
      client_id: "${AZURE_CLIENT_ID}"
      client_secret: "${AZURE_CLIENT_SECRET}"
      allowed_domains:
        - "outlook.com"
      extra_scopes:
        - profile
        - email

    # a sample provider where users with the attribute `wg_admin` set to `true` are considered as admins
    - id: oidc-with-admin-attribute
      provider_name: google
      display_name: Login with</br>Google
      base_url: https://accounts.google.com
      client_id: the-client-id-1234.apps.googleusercontent.com
      client_secret: A_CLIENT_SECRET
      extra_scopes:
        - https://www.googleapis.com/auth/userinfo.email
        - https://www.googleapis.com/auth/userinfo.profile
      field_map:
        user_identifier: sub
        email: email
        firstname: given_name
        lastname: family_name
        phone: phone_number
        department: department
        is_admin: wg_admin
      admin_mapping:
        admin_value_regex: ^true$
      registration_enabled: true
      log_user_info: true

    # a sample provider where users in the group `the-admin-group` are considered as admins
    - id: oidc-with-admin-group
      provider_name: google2
      display_name: Login with</br>Google2
      base_url: https://accounts.google.com
      client_id: another-client-id-1234.apps.googleusercontent.com
      client_secret: A_CLIENT_SECRET
      extra_scopes:
        - https://www.googleapis.com/auth/userinfo.email
        - https://www.googleapis.com/auth/userinfo.profile
      field_map:
        user_identifier: sub
        email: email
        firstname: given_name
        lastname: family_name
        phone: phone_number
        department: department
        user_groups: groups
      admin_mapping:
        admin_group_regex: ^the-admin-group$
      registration_enabled: true
      log_user_info: true
```

## Plain OAuth2 Authentication

```yaml
# ... (basic configuration)

auth:
  oauth:
    # a sample provider where users with the attribute `this-attribute-must-be-true` set to `true` or `True`
    # are considered as admins
    - id: google_plain_oauth-with-admin-attribute
      provider_name: google3
      display_name: Login with</br>Google3
      client_id: another-client-id-1234.apps.googleusercontent.com
      client_secret: A_CLIENT_SECRET
      auth_url: https://accounts.google.com/o/oauth2/v2/auth
      token_url: https://oauth2.googleapis.com/token
      user_info_url: https://openidconnect.googleapis.com/v1/userinfo
      scopes:
        - openid
        - email
        - profile
      field_map:
        user_identifier: sub
        email: email
        firstname: name
        is_admin: this-attribute-must-be-true
      admin_mapping:
        admin_value_regex: ^(True|true)$
      registration_enabled: true
    
    # a sample provider where either users with the attribute `this-attribute-must-be-true` set to `true` or 
    # users in the group `admin-group-name` are considered as admins
    - id: google_plain_oauth_with_groups
      provider_name: google4
      display_name: Login with</br>Google4
      client_id: another-client-id-1234.apps.googleusercontent.com
      client_secret: A_CLIENT_SECRET
      auth_url: https://accounts.google.com/o/oauth2/v2/auth
      token_url: https://oauth2.googleapis.com/token
      user_info_url: https://openidconnect.googleapis.com/v1/userinfo
      scopes:
        - openid
        - email
        - profile
        - i-want-some-groups
      field_map:
        email: email
        firstname: name
        user_identifier: sub
        is_admin: this-attribute-must-be-true
        user_groups: groups
      admin_mapping:
        admin_value_regex: ^true$
        admin_group_regex: ^admin-group-name$
      registration_enabled: true
      log_user_info: true
```
