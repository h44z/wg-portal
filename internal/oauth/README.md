# OAuth2 providers

## How to add a new provider

In this example we will use `gitlab` as the authentication provider.

1) Edit the file `$project_root/internal/oauth/oauthproviders/provider.go` and add a new constant to identify the provider type:

    ```go
    const (
        ...
        ProviderGitlab ProviderType = "gitlab"
    )
    ```

2) Create a new directory in `$project_root/internal/oauth/oauthproviders`:

    ```bash
    $ mkdir $project_root/internal/oauth/oauthproviders/gitlab
    ```

3) Create the new Go file:

    ```bash
    $ touch $project_root/internal/oauth/oauthproviders/gitlab/gitlab.go
    ```

   Create the needed code to make a working provider.

4) Edit the file `$project_root/internal/oauth/config.go`:
   
    - Add the new config section:

        ```go
        type Config struct {
            ...
            Gitlab struct {
                ClientID     string `yaml:"clientID" envconfig:"OAUTH_GITLAB_CLIENT_ID"`
                ClientSecret string `yaml:"clientSecret" envconfig:"OAUTH_GITLAB_CLIENT_SECRET"`
                CreateUsers  bool   `yaml:"createUsers" envconfig:"OAUTH_GITLAB_CREATE_USERS"`
                Enabled      bool   `yaml:"enabled" envconfig:"OAUTH_GITLAB_ENABLED"`
            } `yaml:"gitlab"`
            ...
        }
        ```
    
    - Add the new check in the `IsEnabled()` function:

        ```go
        func (c Config) IsEnabled() bool {
            return c.Github.Enabled ||
                c.Google.Enabled ||
                c.Gitlab.Enabled
        }
        ```

    - Add the new code section in the `NewProviderFromID()` function:

        ```go
        ...
        case oauthproviders.ProviderGitlab:
            config := oauthproviders.ProviderConfig{
                ClientID:     c.Gitlab.ClientID,
                ClientSecret: c.Gitlab.ClientSecret,
                RedirectURL:  redirectURL,
                CreateUsers:  c.Gitlab.CreateUsers,
            }

            return gitlab.New(config), nil
        ...
        ```

5) Edit the file `$project_root/server/configuration.go` to add the new default values:

    ```go
    ...
    cfg.OAUTH.Gitlab.ClientID = "clientid"
    cfg.OAUTH.Gitlab.ClientSecret = "supersecret"
    cfg.OAUTH.Gitlab.Enabled = false
    cfg.OAUTH.Gitlab.CreateUsers = false
    ...
    ```

6) Edit the file `$project_root/server/routes.go` to add the new login route:

    ```go
    if s.config.OAUTH.IsEnabled() || s.config.OIDC.IsEnabled() {
        oauth := s.server.Group("/oauth")
        oauth.Use(csrfMiddleware)
        oauth.GET(s.config.OAUTH.RedirectURL, s.OAuthCallback)
        ...
        ...
        // The new code starts here
           if s.config.OAUTH.Gitlab.Enabled {
               oauth.POST(fmt.Sprintf("/%s/login", oauthproviders.ProviderGitlab), s.OAuthLogin)
           }
    }
    ```

7) Update the file `$project_root/server/handlers_auth.go` adding the needed variable to the html template in the `GetLogin()` function:

    ```go
       c.HTML(http.StatusOK, "login.html", gin.H{
        ...
        "oauthGitlabEnabled": s.config.OAUTH.Gitlab.Enabled,
        ...
    })

    ```

8) Update the file `$project_root/assets/tpl/login.html` addint the new login button:

    ```gotemplate
    ...
    {{ if eq .oauthGitlabEnabled true }}
    <div class="mt-3">
        <form action="/oauth/gitlab/login" method="post">
            <input type="hidden" name="_csrf" value="{{.Csrf}}">
            <button class="btn btn-block btn-social btn-sm btn-gitlab" type="submit"><span class="fa fa-gitlab"></span> Sign in with Gitlab</button>
        </form>
    </div>
    {{end}}
    ...
    ```
   
   You can add/change the css styles as needed in the file `$project_root/assets/css/signin.css`.
