package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/aromaw/browser-session/internal/browser"
	"github.com/aromaw/browser-session/internal/session"
)

var version = "dev"

const usage = `browser-session — independent Chrome / Chromium sessions

Usage:
  browser-session browsers
  browser-session create NAME [--browser chrome|chromium|/path/to/executable]
  browser-session open NAME [https://example.com ...]
  browser-session temp [--browser PATH] [https://example.com ...]
  browser-session list [--json]
  browser-session status NAME
  browser-session close NAME
  browser-session delete NAME --yes
  browser-session cleanup
  browser-session version

Global option (before command): --data-dir PATH
Environment: BROWSER_SESSION_HOME overrides the platform data directory.
Names: lowercase letters, numbers, '-' and '_'; start with a letter.
A browser must be installed separately. No browser data is read or uploaded.
`

func main() {
	if e := run(os.Args[1:], os.Stdout, os.Stderr); e != nil {
		fmt.Fprintln(os.Stderr, "error:", e)
		os.Exit(1)
	}
}
func run(args []string, out, errOut io.Writer) error {
	if handled, err := session.DispatchSupervisor(args); handled {
		return err
	}
	g := flag.NewFlagSet("browser-session", flag.ContinueOnError)
	g.SetOutput(errOut)
	root := g.String("data-dir", os.Getenv("BROWSER_SESSION_HOME"), "data directory")
	if e := g.Parse(args); e != nil {
		if errors.Is(e, flag.ErrHelp) {
			fmt.Fprint(out, usage)
			return nil
		}
		return e
	}
	args = g.Args()
	if len(args) == 0 || args[0] == "help" {
		fmt.Fprint(out, usage)
		return nil
	}
	command := args[0]
	args = args[1:]
	if command == "version" {
		fmt.Fprintln(out, version)
		return nil
	}
	if command == "browsers" {
		if len(args) > 0 {
			return errors.New("browsers takes no arguments")
		}
		for _, b := range (browser.Launcher{}).DetectBrowsers() {
			fmt.Fprintf(out, "%s\t%s\n", b.ID, b.Executable)
		}
		return nil
	}
	s, e := session.NewStore(*root)
	if e != nil {
		return e
	}
	// Cleanup errors do not prevent using unrelated persistent identities.
	if command != "cleanup" {
		for _, n := range s.Cleanup() {
			fmt.Fprintln(errOut, "cleanup deferred:", n)
		}
	}
	switch command {
	case "create":
		if len(args) == 0 {
			return errors.New("create requires NAME")
		}
		name := args[0]
		fs := flag.NewFlagSet("create", flag.ContinueOnError)
		fs.SetOutput(errOut)
		choice := fs.String("browser", "auto", "browser ID or executable")
		if e = fs.Parse(args[1:]); e != nil {
			return e
		}
		if fs.NArg() != 0 {
			return errors.New("unexpected create arguments")
		}
		v, e := s.Create(name, "persistent", *choice)
		if e != nil {
			return e
		}
		fmt.Fprintln(out, "Created", v.Name)
	case "open":
		if len(args) == 0 {
			return errors.New("open requires NAME")
		}
		v, e := s.Open(args[0], args[1:])
		if e != nil {
			return e
		}
		fmt.Fprintln(out, "Opened", v.Name)
	case "temp":
		fs := flag.NewFlagSet("temp", flag.ContinueOnError)
		fs.SetOutput(errOut)
		choice := fs.String("browser", "auto", "browser ID or executable")
		if e = fs.Parse(args); e != nil {
			return e
		}
		if e = browser.ValidateURLs(fs.Args()); e != nil {
			return e
		}
		v, e := s.Create("temporary-"+session.ID()[:12], "temporary", *choice)
		if e != nil {
			return e
		}
		v, e = s.Open(v.Name, fs.Args())
		if e != nil {
			return fmt.Errorf("%s: %w", v.Name, e)
		}
		fmt.Fprintln(out, "Opened", v.Name)
	case "list":
		if len(args) > 1 || len(args) == 1 && args[0] != "--json" {
			return errors.New("usage: list [--json]")
		}
		c, e := s.Read()
		if e != nil {
			return e
		}
		type row struct {
			session.Session
			Status  string `json:"status"`
			DataDir string `json:"dataDir"`
		}
		rows := []row{}
		statuses := s.Statuses(c.Sessions)
		for _, v := range c.Sessions {
			rows = append(rows, row{v, statuses[v.ID], s.DataPath(v)})
		}
		if len(args) > 0 {
			enc := json.NewEncoder(out)
			enc.SetIndent("", "  ")
			return enc.Encode(rows)
		}
		w := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tTYPE\tBROWSER\tSTATUS")
		for _, r := range rows {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", r.Name, r.Type, r.Browser.ID, r.Status)
		}
		return w.Flush()
	case "status":
		if len(args) != 1 {
			return errors.New("status requires NAME")
		}
		v, r, e := s.Detail(args[0])
		if e != nil {
			return e
		}
		fmt.Fprintf(out, "Name: %s\nType: %s\nStatus: %s\nBrowser: %s\nData: %s\n", v.Name, v.Type, s.Status(v), v.Browser.Executable, s.DataPath(v))
		if r.Error != "" {
			fmt.Fprintln(out, "Last notice:", r.Error)
		}
	case "close":
		if len(args) != 1 {
			return errors.New("close requires NAME")
		}
		if e = s.Close(args[0]); e != nil {
			return e
		}
		fmt.Fprintln(out, "Closed", args[0])
	case "delete":
		if len(args) != 2 || args[1] != "--yes" {
			return errors.New("deletion is permanent; use: delete NAME --yes")
		}
		if e = s.Delete(args[0]); e != nil {
			return e
		}
		fmt.Fprintln(out, "Deleted", args[0])
	case "cleanup":
		if len(args) != 0 {
			return errors.New("cleanup takes no arguments")
		}
		notes := s.Cleanup()
		if len(notes) > 0 {
			return errors.New(strings.Join(notes, "; "))
		}
		fmt.Fprintln(out, "Cleanup complete; active sessions retained")
	default:
		return fmt.Errorf("unknown command %q; use help", command)
	}
	return nil
}
