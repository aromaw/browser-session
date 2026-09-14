// Package native provides a narrowly scoped Chrome Native Messaging bridge.
package native

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"

	"github.com/aromaw/browser-session/internal/browser"
	"github.com/aromaw/browser-session/internal/platform"
	"github.com/aromaw/browser-session/internal/session"
)

const HostName = "io.github.aromaw.browser_session"
const MaxMessage = 64 * 1024

var extensionID = regexp.MustCompile(`^[a-p]{32}$`)

type Config struct {
	Origin  string `json:"origin"`
	Root    string `json:"root"`
	Browser string `json:"browser"`
}
type Request struct {
	Op  string `json:"op"`
	URL string `json:"url"`
}
type Response struct {
	OK    bool   `json:"ok"`
	Name  string `json:"name,omitempty"`
	Error string `json:"error,omitempty"`
}

func configPath() (string, error) {
	root, err := platform.DataHome()
	return filepath.Join(root, "native-host.json"), err
}
func Load() (Config, error) {
	var c Config
	p, err := configPath()
	if err != nil {
		return c, err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return c, err
	}
	err = json.Unmarshal(b, &c)
	return c, err
}

// One sendNativeMessage call, one request. No shell, arbitrary arguments,
// filesystem API, Cookie access, or remote configuration is exposed.
func Serve(in io.Reader, out io.Writer, origin string, c Config, launch func(string) (string, error)) error {
	if origin != c.Origin || !validOrigin(origin) {
		return errors.New("extension origin rejected")
	}
	var size uint32
	if err := binary.Read(in, binary.NativeEndian, &size); err != nil {
		return err
	}
	if size == 0 || size > MaxMessage {
		return errors.New("invalid message size")
	}
	data := make([]byte, int(size))
	if _, err := io.ReadFull(in, data); err != nil {
		return err
	}
	var req Request
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	err := dec.Decode(&req)
	var extra any
	if err == nil && dec.Decode(&extra) != io.EOF {
		err = errors.New("trailing JSON")
	}
	response := Response{}
	if err != nil || req.Op != "new-session" || len(req.URL) > 8192 || req.URL == "" || browser.ValidateURLs([]string{req.URL}) != nil {
		response.Error = "只能在普通 HTTP/HTTPS 网页上创建会话。"
	} else {
		name, err := launch(req.URL)
		if err != nil {
			response.Error = "启动未确认。请用 browser-session list/status 检查；不要反复重试。"
		} else {
			response.OK = true
			response.Name = name
		}
	}
	data, err = json.Marshal(response)
	if err != nil {
		return err
	}
	if err = binary.Write(out, binary.NativeEndian, uint32(len(data))); err != nil {
		return err
	}
	_, err = io.Copy(out, bytes.NewReader(data))
	return err
}
func validOrigin(origin string) bool {
	return len(origin) == 52 && origin[:19] == "chrome-extension://" && origin[51:] == "/" && extensionID.MatchString(origin[19:51])
}
func Run(origin string, in io.Reader, out io.Writer) error {
	c, err := Load()
	if err != nil {
		return errors.New("native host not installed")
	}
	return Serve(in, out, origin, c, func(url string) (string, error) {
		s, err := session.NewStore(c.Root)
		if err != nil {
			return "", err
		}
		_ = s.Cleanup()
		v, err := s.Create("temporary-"+session.ID()[:12], "temporary", c.Browser)
		if err != nil {
			return "", err
		}
		_, err = s.Open(v.ID, []string{url})
		return v.Name, err
	})
}
