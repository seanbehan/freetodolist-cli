package main

import (
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
)

const itemsUsage = `freetodolist items - manage items

USAGE
  freetodolist items list <list-uid> [filters/sort]
  freetodolist items create <list-uid> --body=<text> [--due=<RFC3339>] [--tab=<slug>] [--bottom]
  freetodolist items show <item-uid>
  freetodolist items update <item-uid> [--body=...] [--complete=true|false]
                                       [--due=<RFC3339>|none] [--tab=<slug>|none] [--archived=true|false]
  freetodolist items delete <item-uid>

FLAGS (list)
  --archived         include archived items
  --completed        only completed items
  --uncompleted      only uncompleted items
  --tab <slug>       filter by tab slug (or "untabbed")
  --due-before <ts>  RFC3339 timestamp upper bound
  --due-after <ts>   RFC3339 timestamp lower bound
  --sort <field>     position_desc, date_asc, date_desc, created_asc/desc, updated_asc/desc
`

func runItems(g *globalFlags, args []string) error {
	if len(args) == 0 {
		fmt.Fprint(stderr(), itemsUsage)
		return fmt.Errorf("missing subcommand")
	}
	if err := g.requireToken(); err != nil {
		return err
	}
	c := newClient(g.baseURL, g.token)

	switch args[0] {
	case "list", "ls":
		return itemsList(g, c, args[1:])
	case "create", "add":
		return itemsCreate(g, c, args[1:])
	case "show", "get":
		return itemsShow(g, c, args[1:])
	case "update", "edit":
		return itemsUpdate(g, c, args[1:])
	case "delete", "rm":
		return itemsDelete(g, c, args[1:])
	case "-h", "--help", "help":
		fmt.Print(itemsUsage)
		return nil
	default:
		fmt.Fprint(stderr(), itemsUsage)
		return fmt.Errorf("unknown items subcommand: %s", args[0])
	}
}

func itemsList(g *globalFlags, c *client, args []string) error {
	fs := flag.NewFlagSet("items list", flag.ContinueOnError)
	archived := fs.Bool("archived", false, "")
	completed := fs.Bool("completed", false, "")
	uncompleted := fs.Bool("uncompleted", false, "")
	tab := fs.String("tab", "", "")
	dueBefore := fs.String("due-before", "", "")
	dueAfter := fs.String("due-after", "", "")
	sort := fs.String("sort", "", "")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("usage: items list <list-uid>")
	}
	listUID := fs.Arg(0)

	q := url.Values{}
	if *archived {
		q.Set("include_archived", "true")
	}
	if *completed {
		q.Set("only_completed", "true")
	}
	if *uncompleted {
		q.Set("only_uncompleted", "true")
	}
	if *tab != "" {
		q.Set("tab", *tab)
	}
	if *dueBefore != "" {
		q.Set("due_before", *dueBefore)
	}
	if *dueAfter != "" {
		q.Set("due_after", *dueAfter)
	}
	if *sort != "" {
		q.Set("sort", *sort)
	}

	var resp itemsIndexResponse
	if err := c.do("GET", "/api/v1/lists/"+url.PathEscape(listUID)+"/items", q, nil, &resp); err != nil {
		return err
	}

	emit(g, c, func(w io.Writer) {
		if len(resp.Items) == 0 {
			fmt.Fprintln(w, "No items.")
			return
		}
		t := newTable(w)
		fmt.Fprintln(t, "UID\tPOS\tDONE\tDUE\tTAB\tBODY")
		for _, it := range resp.Items {
			fmt.Fprintf(t, "%s\t%d\t%s\t%s\t%s\t%s\n",
				it.UID,
				it.Position,
				boolMark(it.Complete),
				shortDate(derefStr(it.DueAt)),
				derefStr(it.TabSlug),
				truncate(it.Body, 60),
			)
		}
		t.Flush()
	})
	return nil
}

func itemsCreate(g *globalFlags, c *client, args []string) error {
	fs := flag.NewFlagSet("items create", flag.ContinueOnError)
	body := fs.String("body", "", "item text (required)")
	due := fs.String("due", "", "RFC3339 due timestamp")
	tabID := fs.Int("tab-id", 0, "tab id (alternative to --tab)")
	bottom := fs.Bool("bottom", false, "add to bottom (default: top)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("usage: items create <list-uid> --body=<text>")
	}
	if *body == "" {
		return fmt.Errorf("--body is required")
	}
	listUID := fs.Arg(0)

	itemPayload := map[string]any{"body": *body}
	if *due != "" {
		itemPayload["due_at"] = *due
	}
	if *tabID > 0 {
		itemPayload["tab_id"] = *tabID
	}
	payload := map[string]any{"item": itemPayload}

	q := url.Values{}
	if *bottom {
		q.Set("position_at_top", "false")
	}

	var resp itemEnvelope
	path := "/api/v1/lists/" + url.PathEscape(listUID) + "/items"
	if err := c.do("POST", path, q, payload, &resp); err != nil {
		return err
	}
	emitMessage(g, c, fmt.Sprintf("Created item %s", resp.Item.UID))
	return nil
}

func itemsShow(g *globalFlags, c *client, args []string) error {
	fs := flag.NewFlagSet("items show", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("usage: items show <item-uid>")
	}
	uid := fs.Arg(0)

	var resp itemEnvelope
	if err := c.do("GET", "/api/v1/items/"+url.PathEscape(uid), nil, nil, &resp); err != nil {
		return err
	}

	emit(g, c, func(w io.Writer) {
		it := resp.Item
		fmt.Fprintf(w, "%s\n", it.Body)
		fmt.Fprintf(w, "  uid: %s   list: %s   pos: %d\n", it.UID, it.ListUID, it.Position)
		fmt.Fprintf(w, "  done: %v   archived: %v   due: %s   completed_at: %s\n",
			it.Complete, it.Archived, derefStr(it.DueAt), derefStr(it.CompletedAt))
		if it.TabSlug != nil {
			fmt.Fprintf(w, "  tab: %s\n", *it.TabSlug)
		}
		fmt.Fprintf(w, "  created: %s   updated: %s\n", shortDate(it.CreatedAt), shortDate(it.UpdatedAt))
	})
	return nil
}

func itemsUpdate(g *globalFlags, c *client, args []string) error {
	fs := flag.NewFlagSet("items update", flag.ContinueOnError)
	body := fs.String("body", "", "")
	complete := fs.String("complete", "", "true|false")
	due := fs.String("due", "", "RFC3339 due timestamp; pass 'none' to clear")
	tabID := fs.Int("tab-id", -1, "tab id; pass 0 to untab (use -1 to leave alone)")
	archived := fs.String("archived", "", "true|false")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("usage: items update <item-uid> [flags]")
	}
	uid := fs.Arg(0)

	itemPayload := map[string]any{}
	if *body != "" {
		itemPayload["body"] = *body
	}
	if *complete != "" {
		b, err := parseBoolFlag("complete", *complete)
		if err != nil {
			return err
		}
		itemPayload["complete"] = b
	}
	if *archived != "" {
		b, err := parseBoolFlag("archived", *archived)
		if err != nil {
			return err
		}
		itemPayload["archived"] = b
	}
	if *due != "" {
		if *due == "none" {
			itemPayload["due_at"] = nil
		} else {
			itemPayload["due_at"] = *due
		}
	}
	if *tabID == 0 {
		itemPayload["tab_id"] = nil
	} else if *tabID > 0 {
		itemPayload["tab_id"] = *tabID
	}

	if len(itemPayload) == 0 {
		return fmt.Errorf("no fields to update — pass at least one of --body --complete --due --tab-id --archived")
	}

	var resp itemEnvelope
	path := "/api/v1/items/" + url.PathEscape(uid)
	if err := c.do("PATCH", path, nil, map[string]any{"item": itemPayload}, &resp); err != nil {
		return err
	}
	emitMessage(g, c, "Updated item "+resp.Item.UID)
	return nil
}

func itemsDelete(g *globalFlags, c *client, args []string) error {
	fs := flag.NewFlagSet("items delete", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("usage: items delete <item-uid>")
	}
	uid := fs.Arg(0)

	if err := c.do("DELETE", "/api/v1/items/"+url.PathEscape(uid), nil, nil, nil); err != nil {
		return err
	}
	emitMessage(g, c, "Deleted item "+uid)
	return nil
}

func parseBoolFlag(name, v string) (bool, error) {
	switch v {
	case "true", "t", "yes", "y", "1":
		return true, nil
	case "false", "f", "no", "n", "0":
		return false, nil
	default:
		return false, fmt.Errorf("--%s expects true|false, got %q", name, v)
	}
}

// stderr returns os.Stderr; introduced as a tiny shim so test mocks could
// override it later if needed without rewriting every caller.
func stderr() io.Writer { return os.Stderr }
