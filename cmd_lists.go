package main

import (
	"flag"
	"fmt"
	"io"
	"net/url"
)

const listsUsage = `freetodolist lists - manage your lists

USAGE
  freetodolist lists list [--archived] [--sort=<field>]
  freetodolist lists show <list-uid>

FLAGS (list)
  --archived         include archived lists
  --sort <field>     sort by name, position, created_at_desc, updated_at_desc, items_count_desc, etc.
`

func runLists(g *globalFlags, args []string) error {
	if len(args) == 0 {
		fmt.Fprint(stderr(), listsUsage)
		return fmt.Errorf("missing subcommand")
	}
	if err := g.requireToken(); err != nil {
		return err
	}
	c := newClient(g.baseURL, g.token)

	switch args[0] {
	case "list", "ls":
		return listsList(g, c, args[1:])
	case "show", "get":
		return listsShow(g, c, args[1:])
	case "-h", "--help", "help":
		fmt.Print(listsUsage)
		return nil
	default:
		fmt.Fprint(stderr(), listsUsage)
		return fmt.Errorf("unknown lists subcommand: %s", args[0])
	}
}

func listsList(g *globalFlags, c *client, args []string) error {
	fs := flag.NewFlagSet("lists list", flag.ContinueOnError)
	includeArchived := fs.Bool("archived", false, "include archived lists")
	sort := fs.String("sort", "", "sort field")
	if err := fs.Parse(args); err != nil {
		return err
	}

	q := url.Values{}
	if *includeArchived {
		q.Set("include_archived", "true")
	}
	if *sort != "" {
		q.Set("sort", *sort)
	}

	var resp listsIndexResponse
	if err := c.do("GET", "/api/v1/lists", q, nil, &resp); err != nil {
		return err
	}

	emit(g, c, func(w io.Writer) {
		if len(resp.Lists) == 0 {
			fmt.Fprintln(w, "No lists.")
			return
		}
		t := newTable(w)
		fmt.Fprintln(t, "UID\tNAME\tITEMS\tDONE\tOVERDUE\tARCH\tUPDATED")
		for _, l := range resp.Lists {
			fmt.Fprintf(t, "%s\t%s\t%d\t%d\t%d\t%s\t%s\n",
				l.UID,
				truncate(l.Name, 40),
				l.ItemsCount,
				l.CompletedItemsCount,
				l.OverdueItemsCount,
				boolMark(l.Archived),
				shortDate(l.UpdatedAt),
			)
		}
		t.Flush()
	})
	return nil
}

func listsShow(g *globalFlags, c *client, args []string) error {
	fs := flag.NewFlagSet("lists show", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("usage: lists show <list-uid>")
	}
	uid := fs.Arg(0)

	var resp listShowResponse
	if err := c.do("GET", "/api/v1/lists/"+url.PathEscape(uid), nil, nil, &resp); err != nil {
		return err
	}

	emit(g, c, func(w io.Writer) {
		l := resp.List
		fmt.Fprintf(w, "%s  (%s)\n", l.Name, l.UID)
		if l.Description != "" {
			fmt.Fprintln(w, l.Description)
		}
		fmt.Fprintf(w, "  total=%d  done=%d  open=%d  overdue=%d  archived=%d\n",
			l.Stats.TotalItems, l.Stats.CompletedItems, l.Stats.UncompletedItems,
			l.Stats.OverdueItems, l.Stats.ArchivedItems)
		if l.Shareable {
			fmt.Fprintf(w, "  share: %s\n", l.ShareableURL)
		}

		if len(resp.Tabs) > 0 {
			fmt.Fprintln(w, "\nTabs:")
			t := newTable(w)
			fmt.Fprintln(t, "  POS\tSLUG\tNAME")
			for _, tb := range resp.Tabs {
				fmt.Fprintf(t, "  %d\t%s\t%s\n", tb.Position, tb.Slug, tb.Name)
			}
			t.Flush()
		}

		fmt.Fprintln(w, "\nItems:")
		if len(resp.Items) == 0 {
			fmt.Fprintln(w, "  (none)")
			return
		}
		t := newTable(w)
		fmt.Fprintln(t, "  POS\tDONE\tDUE\tTAB\tBODY")
		for _, it := range resp.Items {
			fmt.Fprintf(t, "  %d\t%s\t%s\t%s\t%s\n",
				it.Position,
				boolMark(it.Complete),
				shortDate(derefStr(it.DueAt)),
				derefStr(it.TabSlug),
				truncate(it.Body, 60),
			)
		}
		t.Flush()
		fmt.Fprintln(w, "\nNote: item UIDs are not exposed by `lists show`.")
		fmt.Fprintln(w, "      Use `freetodolist items list "+l.UID+"` to get UIDs for updates.")
	})
	return nil
}
