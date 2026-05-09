package main

import (
	"flag"
	"fmt"
	"io"
	"net/url"
)

// ─── dashboard ─────────────────────────────────────────────────────────────

func runDashboard(g *globalFlags, args []string) error {
	if err := g.requireToken(); err != nil {
		return err
	}
	c := newClient(g.baseURL, g.token)

	fs := flag.NewFlagSet("dashboard", flag.ContinueOnError)
	sort := fs.String("sort", "", "name|created_at|updated_at|position")
	direction := fs.String("direction", "", "asc|desc")
	if err := fs.Parse(args); err != nil {
		return err
	}

	q := url.Values{}
	if *sort != "" {
		q.Set("sort", *sort)
	}
	if *direction != "" {
		q.Set("direction", *direction)
	}

	var resp dashboardResponse
	if err := c.do("GET", "/api/v1/dashboard", q, nil, &resp); err != nil {
		return err
	}

	emit(g, c, func(w io.Writer) {
		s := resp.Stats
		fmt.Fprintf(w, "Lists: %d (%d archived)   Items: %d (%d done, %d%% complete)   Overdue: %d\n",
			s.ListsCount, s.ArchivedListsCount, s.TotalItems, s.CompletedItems,
			s.CompletionPercentage, s.OverdueItemsCount)
		if len(resp.Lists) == 0 {
			fmt.Fprintln(w, "\nNo lists.")
			return
		}
		fmt.Fprintln(w, "")
		t := newTable(w)
		fmt.Fprintln(t, "UID\tNAME\tITEMS\tDONE\tOVERDUE\tUPDATED")
		for _, l := range resp.Lists {
			fmt.Fprintf(t, "%s\t%s\t%d\t%d\t%d\t%s\n",
				l.UID, truncate(l.Name, 40),
				l.ItemsCount, l.CompletedItemsCount,
				l.OverdueItemsCount, shortDate(l.UpdatedAt))
		}
		t.Flush()
	})
	return nil
}

// ─── overdue ───────────────────────────────────────────────────────────────

func runOverdue(g *globalFlags, args []string) error {
	if err := g.requireToken(); err != nil {
		return err
	}
	c := newClient(g.baseURL, g.token)

	fs := flag.NewFlagSet("overdue", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}

	var resp overdueResponse
	if err := c.do("GET", "/api/v1/overdue_items", nil, nil, &resp); err != nil {
		return err
	}

	emit(g, c, func(w io.Writer) {
		fmt.Fprintf(w, "%d overdue items\n", resp.Count)
		if resp.Count == 0 {
			return
		}
		t := newTable(w)
		fmt.Fprintln(t, "DUE\tDAYS\tLIST\tBODY")
		for _, it := range resp.Items {
			fmt.Fprintf(t, "%s\t%d\t%s\t%s\n",
				shortDate(derefStr(it.DueAt)),
				it.DaysOverdue,
				truncate(it.List.Name, 25),
				truncate(it.Body, 60))
		}
		t.Flush()
	})
	return nil
}

// ─── due (next 30 days) ────────────────────────────────────────────────────

func runDue(g *globalFlags, args []string) error {
	if err := g.requireToken(); err != nil {
		return err
	}
	c := newClient(g.baseURL, g.token)

	fs := flag.NewFlagSet("due", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}

	var resp dueResponse
	if err := c.do("GET", "/api/v1/items_due_dates", nil, nil, &resp); err != nil {
		return err
	}

	emit(g, c, func(w io.Writer) {
		fmt.Fprintf(w, "%d items due in the next 30 days\n", resp.Count)
		if resp.Count == 0 {
			return
		}
		t := newTable(w)
		fmt.Fprintln(t, "DUE\tDAYS\tLIST\tBODY")
		for _, it := range resp.Items {
			fmt.Fprintf(t, "%s\t%d\t%s\t%s\n",
				shortDate(derefStr(it.DueAt)),
				it.DaysUntilDue,
				truncate(it.List.Name, 25),
				truncate(it.Body, 60))
		}
		t.Flush()
	})
	return nil
}

// ─── shared (no auth required) ─────────────────────────────────────────────

func runShared(g *globalFlags, args []string) error {
	fs := flag.NewFlagSet("shared", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("usage: shared <token>")
	}
	token := fs.Arg(0)

	// shared endpoint accepts no auth; pass empty token.
	c := newClient(g.baseURL, "")

	var resp sharedListResponse
	if err := c.do("GET", "/api/v1/shared/"+url.PathEscape(token), nil, nil, &resp); err != nil {
		return err
	}

	emit(g, c, func(w io.Writer) {
		l := resp.List
		fmt.Fprintf(w, "%s\n", l.Name)
		if l.Description != "" {
			fmt.Fprintln(w, l.Description)
		}
		fmt.Fprintf(w, "  total=%d  done=%d  open=%d  overdue=%d\n",
			l.Stats.TotalItems, l.Stats.CompletedItems,
			l.Stats.UncompletedItems, l.Stats.OverdueItems)
		fmt.Fprintln(w, "")
		if len(resp.Items) == 0 {
			fmt.Fprintln(w, "(no items)")
			return
		}
		t := newTable(w)
		fmt.Fprintln(t, "POS\tDONE\tDUE\tBODY")
		for _, it := range resp.Items {
			fmt.Fprintf(t, "%d\t%s\t%s\t%s\n",
				it.Position,
				boolMark(it.Complete),
				shortDate(derefStr(it.DueAt)),
				truncate(it.Body, 60))
		}
		t.Flush()
	})
	return nil
}
