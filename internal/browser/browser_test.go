package browser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildArgsAndInjection(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "spaces and 中文")
	a, e := (Launcher{}).BuildArgs(dir, []string{"https://example.com/?q=a%20b"})
	if e != nil {
		t.Fatal(e)
	}
	if a[0] != "--user-data-dir="+dir || a[1] != "--disk-cache-dir="+filepath.Join(dir, "Cache") {
		t.Fatal(a)
	}
	for _, arg := range a {
		if strings.HasPrefix(arg, "--remote-debugging") || arg == "--no-sandbox" || arg == "--incognito" {
			t.Fatal("unsafe default", arg)
		}
	}
	for _, bad := range []string{"--user-data-dir=/bad", "javascript:alert(1)", "file:///etc/passwd", "https://user:password@example.com", "example.com"} {
		if _, e = (Launcher{}).BuildArgs(dir, []string{bad}); e == nil {
			t.Fatal("accepted", bad)
		}
	}
}
func TestManualExecutable(t *testing.T) {
	exe, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	b, e := (Launcher{}).ResolveExecutable(exe)
	if e != nil || b.Executable == "" {
		t.Fatal(b, e)
	}
}
