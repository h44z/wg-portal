WireGuard Portal sends emails when you share a configuration with a user. 
By default, the application uses embedded templates. You can fully customize these emails by pointing the Portal 
to a folder containing your own templates. If the folder is empty on startup, the default embedded templates 
are written there to get you started.

## Configuration

To enable custom templates, set the `mail.templates_path` option in the application configuration file 
or the `WG_PORTAL_MAIL_TEMPLATES_PATH` environment variable to a valid folder path.

For example:

```yaml
mail:
  # ... other mail options ...
  # Path where custom email templates (.gotpl and .gohtml) are stored.
  # If the directory is empty on startup, the default embedded templates
  # will be written there so you can modify them.
  # Leave empty to use embedded templates only.
  templates_path: "/opt/wg-portal/mail-templates"
```

## Template files and names

The system expects the following template names. Place files with these names in your `templates_path` to override the defaults.
You do not need to override all templates, only the ones you want to customize should be present.

- Text templates (`.gotpl`):
  - `mail_with_link.gotpl`
  - `mail_with_attachment.gotpl`
- HTML templates (`.gohtml`):
  - `mail_with_link.gohtml`
  - `mail_with_attachment.gohtml`

Both [text](https://pkg.go.dev/text/template) and [HTML templates](https://pkg.go.dev/html/template) are standard Go 
templates and receive the following data fields, depending on the email type:

- Common fields:
  - `PortalUrl` (string) - external URL of the Portal
  - `PortalName` (string) - site title/company name
  - `User` (*domain.User) - the recipient user (may be partially populated when sending to a peer email)
- Link email (`mail_with_link.*`):
  - `Link` (string) - the download link
- Attachment email (`mail_with_attachment.*`):
  - `ConfigFileName` (string) - filename of the attached WireGuard config
  - `QrcodePngName` (string) - CID content-id of the embedded QR code image

Tip: You can inspect the embedded templates in the repository under [`internal/app/mail/tpl_files/`](https://github.com/h44z/wg-portal/tree/master/internal/app/mail/tpl_files) for reference. 
When the directory at `templates_path` is empty, these files are copied to your folder so you can edit them in place.
