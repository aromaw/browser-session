package native

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"testing"
)

const testOrigin = "chrome-extension://aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/"

func frame(s string) []byte {
	var b bytes.Buffer
	_ = binary.Write(&b, binary.NativeEndian, uint32(len(s)))
	b.WriteString(s)
	return b.Bytes()
}
func TestProtocolBoundary(t *testing.T) {
	for _, data := range []string{`{"op":"new-session","url":"https://example.com/path"}`, `{"op":"new-session","url":"https://example.com/中文"}`} {
		var out bytes.Buffer
		calls := 0
		err := Serve(bytes.NewReader(frame(data)), &out, testOrigin, Config{Origin: testOrigin}, func(url string) (string, error) { calls++; return "temporary-test", nil })
		if err != nil || calls != 1 {
			t.Fatal(err, calls)
		}
		var size uint32
		if err = binary.Read(&out, binary.NativeEndian, &size); err != nil {
			t.Fatal(err)
		}
		if int(size) != out.Len() {
			t.Fatal("wrong byte length")
		}
		var r Response
		if json.Unmarshal(out.Bytes(), &r) != nil || !r.OK || r.Name != "temporary-test" {
			t.Fatal(out.String())
		}
	}
}
func TestRejectBeforeLaunch(t *testing.T) {
	for _, data := range []string{`{"op":"delete","url":"https://example.com"}`, `{"op":"new-session","url":"file:///tmp/a"}`, `{"op":"new-session","url":"https://a:b@example.com"}`, `{"op":"new-session","url":"https://example.com","args":["--disable-web-security"]}`, `{} {}`, `not json`} {
		var out bytes.Buffer
		if err := Serve(bytes.NewReader(frame(data)), &out, testOrigin, Config{Origin: testOrigin}, func(string) (string, error) { t.Fatal("invalid input launched browser"); return "", nil }); err != nil {
			t.Fatal(err)
		}
		var r Response
		_ = json.Unmarshal(out.Bytes()[4:], &r)
		if r.OK || r.Error == "" {
			t.Fatal(r)
		}
	}
	for _, data := range [][]byte{{}, frame("abc")[:5], {255, 255, 255, 127}} {
		if Serve(bytes.NewReader(data), io.Discard, testOrigin, Config{Origin: testOrigin}, nil) == nil {
			t.Fatal("malformed frame accepted")
		}
	}
	if Serve(bytes.NewReader(frame(`{}`)), io.Discard, "chrome-extension://bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb/", Config{Origin: testOrigin}, nil) == nil {
		t.Fatal("wrong extension accepted")
	}
}
func TestErrorDoesNotLeakURLOrInternalDetails(t *testing.T) {
	var out bytes.Buffer
	err := Serve(bytes.NewReader(frame(`{"op":"new-session","url":"https://example.com/?token=secret"}`)), &out, testOrigin, Config{Origin: testOrigin}, func(string) (string, error) { return "", errors.New("token=secret") })
	if err != nil || bytes.Contains(out.Bytes(), []byte("secret")) {
		t.Fatal(err, out.String())
	}
}
func TestManifestLocations(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	for _, os := range []string{"darwin", "linux"} {
		p := ManifestPath(os, home, "", "", home)
		if !filepath.IsAbs(p) || filepath.Base(p) != HostName+".json" {
			t.Fatal(p)
		}
		custom := filepath.Join(home, "Chrome data")
		if ManifestPath(os, home, "", custom, home) != filepath.Join(custom, "NativeMessagingHosts", HostName+".json") {
			t.Fatal("custom path ignored")
		}
	}
}
