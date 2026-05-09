package main

import (
	"flag"
	"fmt"
	"io"
	"net/url"
)

const tabsUsage = `freetodolist tabs - manage list tabs

USAGE
  freetodolist tabs list <list-uid>
  freetodolist tabs create <list-uid> --name=<name>
  freetodolist tabs update <list-uid> <slug> --name=<name>
  freetodolist tabs delete <list-uid> <slug>
  freetodolist tabs sort <list-uid> <slug1,slug2,...>
  freetodolist tabs assign <list-uid> --items=<id,id,...> [--tab=<slug>]

  When 'tabs assign' is run without --tab, items are moved to "All" (untabbed).
`

func runTabs(g *globalFlags, args []string) error {
	if len(args) == 0 {
		fmt.Fprint(stderr(), tabsUsage)
		return fmt.Errorf("missing subcommand")
	}
	if err := g.requireToken(); err != nil {
		return err
	}
	c := newClient(g.baseURL, g.token)

	switch args[0] {
	case "list", "ls":
		return tabsList(g, c, args[1:])
	case "create", "add":
		return tabsCreate(g, c, args[1:])
	case "update", "edit":
		return tabsUpdate(g, c, args[1:])
	case "delete", "rm":
		return tabsDelete(g, c, args[1:])
	case "sort":
		return tabsSort(g, c, args[1:])
	case "assign":
		return tabsAssign(g, c, args[1:])
	case "-h", "--help", "help":
		fmt.Print(tabsUsage)
		return nil
	default:
		fmt.Fprint(stderr(), tabsUsage)
		return fmt.Errorf("unknown tabs subcommand: %s", args[0])
	}
}

func tabsList(g *globalFlags, c *client, args []string) error {
	fs := flag.NewFlagSet("tabs list", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("usage: tabs list <list-uid>")
	}
	listUID := fs.Arg(0)

	var resp tabsIndexResponse
	if err := c.do("GET", "/api/v1/lists/"+url.PathEscape(listUID)+"/tabs", nil, nil, &resp); err != nil {
		return err
	}

	emit(g, c, func(w io.Writer) {
		t := newTable(w)
		fmt.Fprintln(t, "POS\tID\tSLUG\tNAME\tITEMS")
		for _, tb := range resp.Tabs {
			fmt.Fprintf(t, "%d\t%d\t%s\t%s\t%d\n", tb.Position, tb.ID, tb.Slug, tb.Name, derefInt(tb.ItemCount))
		}
		t.Flush()
		fmt.Fprintf(w, "\nUntabbed items: %d\n", resp.UntabbedCount)
	})
	return nil
}

func tabsCreate(g *globalFlags, c *client, args []string) error {
	fs := flag.NewFlagSet("tabs create", flag.ContinueOnError)
	name := fs.String("name", "", "tab name (required)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 1 || *name == "" {
		return fmt.Errorf("usage: tabs create <list-uid> --name=<name>")
	}
	listUID := fs.Arg(0)

	payload := map[string]any{"tab": map[string]any{"name": *name}}

	var resp tabEnvelope
	if err := c.do("POST", "/api/v1/lists/"+url.PathEscape(listUID)+"/tabs", nil, payload, &resp); err != nil {
		return err
	}
	emitMessage(g, c, fmt.Sprintf("Created tab %s (%s)", resp.Tab.Name, resp.Tab.Slug))
	return nil
}

func tabsUpdate(g *globalFlags, c *client, args []string) error {
	fs := flag.NewFlagSet("tabs update", flag.ContinueOnError)
	name := fs.String("name", "", "new tab name (required)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 2 || *name == "" {
		return fmt.Errorf("usage: tabs update <list-uid> <slug> --name=<name>")
	}
	listUID, slug := fs.Arg(0), fs.Arg(1)

	payload := map[string]any{"tab": map[string]any{"name": *name}}

	var resp tabEnvelope
	path := "/api/v1/lists/" + url.PathEscape(listUID) + "/tabs/" + url.PathEscape(slug)
	if err := c.do("PATCH", path, nil, payload, &resp); err != nil {
		return err
	}
	emitMessage(g, c, fmt.Sprintf("Updated tab %s -> %s", slug, resp.Tab.Slug))
	return nil
}

func tabsDelete(g *globalFlags, c *client, args []string) error {
	fs := flag.NewFlagSet("tabs delete", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 2 {
		return fmt.Errorf("usage: tabs delete <list-uid> <slug>")
	}
	listUID, slug := fs.Arg(0), fs.Arg(1)

	path := "/api/v1/lists/" + url.PathEscape(listUID) + "/tabs/" + url.PathEscape(slug)
	if err := c.do("DELETE", path, nil, nil, nil); err != nil {
		return err
	}
	emitMessage(g, c, "Deleted tab "+slug)
	return nil
}

func tabsSort(g *globalFlags, c *client, args []string) error {
	fs := flag.NewFlagSet("tabs sort", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 2 {
		return fmt.Errorf("usage: tabs sort <list-uid> <slug1,slug2,...>")
	}
	listUID := fs.Arg(0)
	slugs := parseSlugList(fs.Arg(1))
	if len(slugs) == 0 {
		return fmt.Errorf("no slugs provided")
	}

	payload := map[string]any{"order": slugs}
	path := "/api/v1/lists/" + url.PathEscape(listUID) + "/tabs/sort"
	if err := c.do("PATCH", path, nil, payload, nil); err != nil {
		return err
	}
	emitMessage(g, c, fmt.Sprintf("Reordered %d tabs", len(slugs)))
	return nil
}

func tabsAssign(g *globalFlags, c *client, args []string) error {
	fs := flag.NewFlagSet("tabs assign", flag.ContinueOnError)
	itemsArg := fs.String("items", "", "comma-separated item IDs (numeric DB ids per the API)")
	tabSlug := fs.String("tab", "", "tab slug (omit to move items to All / untabbed)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("usage: tabs assign <list-uid> --items=<id,id,...> [--tab=<slug>]")
	}
	listUID := fs.Arg(0)
	ids, err := parseIDList(*itemsArg)
	if err != nil {
		return err
	}

	payload := map[string]any{"item_ids": ids}
	if *tabSlug != "" {
		payload["tab_slug"] = *tabSlug
	}

	path := "/api/v1/lists/" + url.PathEscape(listUID) + "/tabs/assign_items"
	if err := c.do("PATCH", path, nil, payload, nil); err != nil {
		return err
	}
	if *tabSlug == "" {
		emitMessage(g, c, fmt.Sprintf("Untabbed %d items", len(ids)))
	} else {
		emitMessage(g, c, fmt.Sprintf("Assigned %d items to tab %s", len(ids), *tabSlug))
	}
	return nil
}
