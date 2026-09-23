package auth

import (
	"context"
	"fmt"
	"strconv"

	"github.com/google/go-github/v88/github"
	"golang.org/x/oauth2"
)

// GitHub OAuth endpoints. Declared inline rather than importing
// golang.org/x/oauth2/github, whose package name (`github`) collides with
// google/go-github.
const (
	githubAuthURL  = "https://github.com/login/oauth/authorize"
	githubTokenURL = "https://github.com/login/oauth/access_token" //nolint:gosec // OAuth endpoint URL, not a credential.
)

// GitHubProvider authenticates site users via GitHub OAuth. It is the default
// provider and, unlike the OIDC providers, needs no discovery.
type GitHubProvider struct {
	oauth *oauth2.Config
}

var _ Provider = (*GitHubProvider)(nil)

// NewGitHubProvider builds the GitHub OAuth provider. redirectURL is the
// absolute /auth/callback URL registered with the OAuth app.
func NewGitHubProvider(clientID, clientSecret, redirectURL string) *GitHubProvider {
	return &GitHubProvider{oauth: &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Scopes:       []string{"read:user", "user:email"},
		Endpoint:     oauth2.Endpoint{AuthURL: githubAuthURL, TokenURL: githubTokenURL},
	}}
}

// Name returns the provider key "github".
func (*GitHubProvider) Name() string { return "github" }

// AuthCodeURL returns the GitHub authorize URL carrying state.
func (g *GitHubProvider) AuthCodeURL(state string) string {
	return g.oauth.AuthCodeURL(state, oauth2.AccessTypeOnline)
}

// Exchange trades the code for a token, then reads the authenticated user via
// the GitHub API to build the Identity. GitHub has no id_token, so identity
// comes from GET /user (+ /user/emails when the profile email is private).
func (g *GitHubProvider) Exchange(ctx context.Context, code string) (*Identity, error) {
	tok, err := g.oauth.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("github token exchange: %w", err)
	}

	client, err := github.NewClient(github.WithAuthToken(tok.AccessToken))
	if err != nil {
		return nil, fmt.Errorf("github client: %w", err)
	}
	user, _, err := client.Users.Get(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("github get user: %w", err)
	}

	email := user.GetEmail()
	if email == "" {
		if email, err = primaryEmail(ctx, client); err != nil {
			return nil, err
		}
	}

	return &Identity{
		Provider: g.Name(),
		Subject:  strconv.FormatInt(user.GetID(), 10),
		Email:    email,
		Login:    user.GetLogin(),
	}, nil
}

// primaryEmail returns the user's primary verified email, or "" if none is
// exposed. An absent email is acceptable — Identity.Email is optional.
func primaryEmail(ctx context.Context, client *github.Client) (string, error) {
	emails, _, err := client.Users.ListEmails(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("github list emails: %w", err)
	}
	for _, e := range emails {
		if e.GetPrimary() && e.GetVerified() {
			return e.GetEmail(), nil
		}
	}
	return "", nil
}
