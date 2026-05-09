package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"
)

// emit prints output for a command. In --json mode it pretty-prints the raw
// response body the server sent us; otherwise it calls human() to render a
// table or summary.
func emit(g *globalFlags, c *client, human func(io.Writer)) {
	if g.json {
		printJSON(os.Stdout, c.lastBody)
		return
	}
	human(os.Stdout)
}

// emitMessage prints a status line for write operations (e.g. "Item created").
// In --json mode it still dumps the raw response so scripts can read it.
func emitMessage(g *globalFlags, c *client, msg string) {
	if g.json {
		printJSON(os.Stdout, c.lastBody)
		return
	}
	fmt.Println(msg)
}

func printJSON(w io.Writer, body []byte) {
	var v any
	if err := json.Unmarshal(body, &v); err != nil {
		// Not JSON — print as-is.
		w.Write(body)
		fmt.Fprintln(w)
		return
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

// newTable builds a tab-aligned writer; caller must call Flush.
func newTable(w io.Writer) *tabwriter.Writer {
	return tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
}

// shortDate parses an RFC3339 timestamp and renders it as YYYY-MM-DD. Returns
// the original string on parse failure so we never silently lose data.
func shortDate(s string) string {
	if s == "" {
		return ""
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		// Try a couple of common alternatives Rails may emit.
		for _, layout := range []string{time.RFC3339Nano, "2006-01-02T15:04:05Z", "2006-01-02"} {
			if t2, err2 := time.Parse(layout, s); err2 == nil {
				t = t2
				err = nil
				break
			}
		}
		if err != nil {
			return s
		}
	}
	return t.Format("2006-01-02")
}

// truncate clamps s to n runes, adding an ellipsis if it had to cut.
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n <= 1 {
		return string(r[:n])
	}
	return string(r[:n-1]) + "…"
}

// derefStr returns *p or "" when p is nil — convenient for *string fields.
func derefStr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// derefInt returns *p or 0 when p is nil.
func derefInt(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

// boolMark renders a check/dash for a boolean column.
func boolMark(b bool) string {
	if b {
		return "✓"
	}
	return "·"
}

// parseIDList splits a comma-separated list of integers (e.g. "1,2,3") into
// a slice. Used by `tabs assign --items=...`.
func parseIDList(s string) ([]int, error) {
	if strings.TrimSpace(s) == "" {
		return nil, fmt.Errorf("empty id list")
	}
	parts := strings.Split(s, ",")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil, fmt.Errorf("invalid id %q: %w", p, err)
		}
		out = append(out, n)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no ids provided")
	}
	return out, nil
}

// parseSlugList splits a comma-separated list of tab slugs.
func parseSlugList(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
