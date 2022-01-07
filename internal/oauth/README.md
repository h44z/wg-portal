# OAuth2 providers

## How to add a new provider

In this example we will use `gitlab` as the authentication provider.


1) Create a new directory in `$project_root/internal/oauth/oauthproviders`:

    ```bash
    $ mkdir $project_root/internal/oauth/oauthproviders/gitlab
    ```

2) Create the new Go file:

    ```bash
    $ touch $project_root/internal/oauth/oauthproviders/gitlab/gitlab.go
    ```

    In the newly created file, define a new constant to identify the provider type:

    ```go
    const ProviderGitlab ProviderType = "gitlab"
    ```

   Then create the needed code to make a working provider (use the other providers as reference).


3) Edit the file `$project_root/internal/oauth/config.go`:
   
    - Add the new config section:

        ```go
        type Config struct {
            ...
            Gitlab struct {
                ClientID     string `yaml:"clientID" envconfig:"OAUTH_GITLAB_CLIENT_ID"`
                ClientSecret string `yaml:"clientSecret" envconfig:"OAUTH_GITLAB_CLIENT_SECRET"`
                CreateUsers  bool   `yaml:"createUsers" envconfig:"OAUTH_GITLAB_CREATE_USERS"`
                Enabled      bool   `yaml:"enabled" envconfig:"OAUTH_GITLAB_ENABLED"`
                provider     oauthproviders.Provider
            } `yaml:"gitlab"`
            ...
        }
        ```
    
    - Add the new initialization code in the `Parse(redirectURL string)` function:

        ```go
        ...
        if c.Gitlab.Enabled {
            c.Gitlab.provider = gitlab.New(oauthproviders.ProviderConfig{
                ClientID:     c.Gitlab.ClientID,
                ClientSecret: c.Gitlab.ClientSecret,
                RedirectURL:  redirectURL,
                CreateUsers:  c.Gitlab.CreateUsers,
            })
            c.enabled = true
        }
        ...
        ```

    - Add the new code section in the `ProviderByID(providerID string)` function:

        ```go
        ...
        switch oauthproviders.ProviderType(providerID) {
        ...
        case gitlab.ProviderGitlab:
            return c.Gitlab.provider, nil
        ...
        ```

    - Add the new code section in the `ToFrontendButtons()` function:

        ```go
        ...
        if c.Gitlab.Enabled {
            fc = append(fc, FrontendButtonConfig{
                ProviderID:  c.Gitlab.provider.ID(),
                ButtonStyle: "btn-gitlab",
                IconStyle:   "fa-gitlab",
                Label:       "Sign in with Gitlab",
            })
        }
        ...
        ```

      > Note: if needed, you can add/change the css styles in the file `$project_root/assets/css/signin.css`.

5) Edit the file `$project_root/server/configuration.go` to add the new default values:

    ```go
    ...
    cfg.OAUTH.Gitlab.ClientID = "clientid"
    cfg.OAUTH.Gitlab.ClientSecret = "supersecret"
    cfg.OAUTH.Gitlab.Enabled = false
    cfg.OAUTH.Gitlab.CreateUsers = false
    ...
    ```
