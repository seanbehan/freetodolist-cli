package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
)

const usage = `freetodolist - CLI for freetodolist.com

USAGE
  freetodolist [global flags] <command> [args]

GLOBAL FLAGS
  --token <token>      API token (or set FREETODOLIST_TOKEN)
  --base-url <url>     API base URL (default https://freetodolist.com,
                       or set FREETODOLIST_BASE_URL)
  --json               Print raw JSON instead of human-readable output

COMMANDS
  login      OAuth log in (opens browser, saves token to ~/.config/freetodolist/credentials.json)
  logout     forget the saved token for this base URL
  whoami     show the user the current token resolves to

  lists      list / show your todo lists
  items      list / create / update / delete items
  tabs       manage list tabs
  dashboard  account-wide stats
  overdue    overdue items across all lists
  due        items due in the next 30 days
  shared     read a publicly shared list by token

TOKEN RESOLUTION
  --token flag > FREETODOLIST_TOKEN env > saved credentials file

Run 'freetodolist <command> -h' for command-specific help.
Run 'freetodolist --help' to see this message.
`

type globalFlags struct {
	token   string
	baseURL string
	json    bool
}

func main() {
	g := &globalFlags{}

	// Parse global flags up to the subcommand. We use a custom loop instead
	// of one big FlagSet so each subcommand can own its own flags cleanly.
	args := os.Args[1:]
	rest, err := parseGlobalFlags(args, g)
	if errors.Is(err, flag.ErrHelp) {
		fmt.Print(usage)
		return
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(2)
	}

	if len(rest) == 0 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}

	g.applyEnvDefaults()

	cmd, sub := rest[0], rest[1:]
	var runErr error

	switch cmd {
	case "-h", "--help", "help":
		fmt.Print(usage)
		return
	case "login":
		runErr = runLogin(g, sub)
	case "logout":
		runErr = runLogout(g, sub)
	case "whoami":
		runErr = runWhoami(g, sub)
	case "lists":
		runErr = runLists(g, sub)
	case "items":
		runErr = runItems(g, sub)
	case "tabs":
		runErr = runTabs(g, sub)
	case "dashboard":
		runErr = runDashboard(g, sub)
	case "overdue":
		runErr = runOverdue(g, sub)
	case "due":
		runErr = runDue(g, sub)
	case "shared":
		runErr = runShared(g, sub)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", cmd)
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}

	if runErr != nil {
		fmt.Fprintln(os.Stderr, "error:", runErr)
		os.Exit(1)
	}
}

// parseGlobalFlags consumes leading global flags from args and returns the
// remainder (subcommand + its args). Stops at the first non-flag token.
func parseGlobalFlags(args []string, g *globalFlags) ([]string, error) {
	fs := flag.NewFlagSet("freetodolist", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.StringVar(&g.token, "token", "", "")
	fs.StringVar(&g.baseURL, "base-url", "", "")
	fs.BoolVar(&g.json, "json", false, "")
	// Suppress default usage; we print our own.
	fs.Usage = func() {}

	// flag.Parse stops at the first non-flag arg, which is the subcommand.
	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	return fs.Args(), nil
}

func (g *globalFlags) applyEnvDefaults() {
	if g.token == "" {
		g.token = os.Getenv("FREETODOLIST_TOKEN")
	}
	if g.baseURL == "" {
		g.baseURL = os.Getenv("FREETODOLIST_BASE_URL")
	}
	if g.baseURL == "" {
		g.baseURL = "https://freetodolist.com"
	}
	// Last resort: pick up a saved access token from the credentials file
	// for the resolved baseURL. Errors here are non-fatal — a missing or
	// unreadable file just means the user hasn't logged in yet.
	if g.token == "" {
		if c, err := loadCredentialsFor(g.baseURL); err == nil && c != nil {
			g.token = c.AccessToken
		}
	}
}

// requireToken errors out if no API token has been provided. The shared-list
// command is the only path that doesn't need one.
func (g *globalFlags) requireToken() error {
	if g.token == "" {
		return fmt.Errorf("API token required: pass --token or set FREETODOLIST_TOKEN")
	}
	return nil
}
