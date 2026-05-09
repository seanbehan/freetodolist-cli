package main

import (
	"context"
	"flag"
	"fmt"
	"io"
)

func runLogin(g *globalFlags, args []string) error {
	fs := flag.NewFlagSet("login", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}

	fmt.Printf("Logging in to %s\n", g.baseURL)

	ctx := context.Background()
	token, clientID, err := oauthLogin(ctx, g.baseURL)
	if err != nil {
		return err
	}

	cred := credentials{
		BaseURL:     g.baseURL,
		AccessToken: token,
		ClientID:    clientID,
	}
	if err := saveCredentialsFor(cred); err != nil {
		return fmt.Errorf("save credentials: %w", err)
	}

	path, _ := credentialsPath()
	fmt.Printf("Saved access token for %s\n  → %s\n", g.baseURL, path)

	// Sanity-check: round-trip /api/v1/me so the user sees a confirmation.
	c := newClient(g.baseURL, token)
	var me meResponse
	if err := c.do("GET", "/api/v1/me", nil, nil, &me); err != nil {
		fmt.Println("Note: /api/v1/me check failed —", err)
		return nil
	}
	fmt.Printf("Logged in as %s", me.User.Email)
	if me.User.Name != "" {
		fmt.Printf(" (%s)", me.User.Name)
	}
	fmt.Println()
	return nil
}

func runLogout(g *globalFlags, args []string) error {
	fs := flag.NewFlagSet("logout", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}

	deleted, err := deleteCredentialsFor(g.baseURL)
	if err != nil {
		return err
	}
	if !deleted {
		fmt.Printf("No saved credentials for %s.\n", g.baseURL)
		return nil
	}
	fmt.Printf("Removed saved credentials for %s.\n", g.baseURL)
	fmt.Println("Note: this only forgets the token locally; it remains valid on the server.")
	fmt.Println("      Revoke it from /admin/oauths or by regenerating your api_token in /profile/edit.")
	return nil
}

func runWhoami(g *globalFlags, args []string) error {
	fs := flag.NewFlagSet("whoami", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := g.requireToken(); err != nil {
		return err
	}

	c := newClient(g.baseURL, g.token)
	var me meResponse
	if err := c.do("GET", "/api/v1/me", nil, nil, &me); err != nil {
		return err
	}
	emit(g, c, func(w io.Writer) {
		if me.User.Name != "" {
			fmt.Fprintf(w, "%s <%s>  (id=%d)\n", me.User.Name, me.User.Email, me.User.ID)
		} else {
			fmt.Fprintf(w, "%s  (id=%d)\n", me.User.Email, me.User.ID)
		}
	})
	return nil
}

type meResponse struct {
	User struct {
		ID    int    `json:"id"`
		Email string `json:"email"`
		Name  string `json:"name"`
	} `json:"user"`
}
