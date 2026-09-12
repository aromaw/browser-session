// Package desktop is the small, testable boundary between the local UI and the
// same session manager used by the CLI. It never serves profile files.
package desktop

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/aromaw/browser-session/internal/browser"
	"github.com/aromaw/browser-session/internal/session"
)

type Row struct {
	session.Session
	Status  string `json:"status"`
	DataDir string `json:"dataDir"`
	Notice  string `json:"notice"`
}
type State struct {
	Rows    []Row  `json:"rows"`
	Root    string `json:"root"`
	Version string `json:"version"`
}
type App struct {
	store     *session.Store
	initErr   error
	version   string
	picker    func() (string, error)
	cleanupMu sync.Mutex
}

func New(root, version string, picker func() (string, error)) *App {
	s, err := session.NewStore(root)
	return &App{store: s, initErr: err, version: version, picker: picker}
}
func (a *App) Snapshot() (State, error) {
	state := State{Rows: []Row{}, Version: a.version}
	if a.initErr != nil {
		return state, a.initErr
	}
	state.Root = a.store.Root
	config, err := a.store.Read()
	if err != nil {
		return state, err
	}
	statuses := a.store.Statuses(config.Sessions)
	for _, v := range config.Sessions {
		notice, err := a.store.Notice(v.ID)
		if err != nil {
			notice = "状态可能已更新，请刷新。"
		}
		state.Rows = append(state.Rows, Row{Session: v, Status: statuses[v.ID], DataDir: a.store.DataPath(v), Notice: notice})
	}
	return state, nil
}
func (a *App) Browsers() []browser.Browser {
	values := (browser.Launcher{}).DetectBrowsers()
	if values == nil {
		return []browser.Browser{}
	}
	return values
}
func (a *App) PickBrowser() (string, error) {
	if a.picker == nil {
		return "", errors.New("文件选择器不可用，请手工填写路径。")
	}
	path, err := a.picker()
	if err != nil || path == "" {
		return path, err
	}
	b, err := (browser.Launcher{}).ResolveExecutable(path)
	return b.Executable, err
}
func targetURLs(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	urls := []string{raw}
	return urls, browser.ValidateURLs(urls)
}
func (a *App) CreateSession(name, kind, choice, url string, launch bool) (session.Session, error) {
	if a.initErr != nil {
		return session.Session{}, a.initErr
	}
	urls, err := targetURLs(url)
	if err != nil {
		return session.Session{}, err
	}
	if kind == "temporary" {
		name = "temporary-" + session.ID()[:12]
		launch = true
	}
	v, err := a.store.Create(strings.TrimSpace(name), kind, strings.TrimSpace(choice))
	if err != nil {
		return v, err
	}
	if launch {
		if _, err = a.store.Open(v.ID, urls); err != nil {
			return v, fmt.Errorf("已创建 %s，但启动未确认：%w", v.Name, err)
		}
	}
	return v, nil
}
func (a *App) OpenSession(id, url string) error {
	if a.initErr != nil {
		return a.initErr
	}
	urls, err := targetURLs(url)
	if err != nil {
		return err
	}
	_, err = a.store.Open(id, urls)
	return err
}
func (a *App) CloseSession(id string) error {
	if a.initErr != nil {
		return a.initErr
	}
	return a.store.Close(id)
}
func (a *App) DeleteSession(id, confirmation string) error {
	if a.initErr != nil {
		return a.initErr
	}
	v, err := a.store.Lookup(id)
	if err != nil {
		return err
	}
	if confirmation != v.Name {
		return errors.New("请输入完整的会话名称确认删除。")
	}
	return a.store.Delete(v.ID)
}
func (a *App) Cleanup() ([]string, error) {
	if a.initErr != nil {
		return nil, a.initErr
	}
	if !a.cleanupMu.TryLock() {
		return nil, errors.New("清理正在进行中。")
	}
	defer a.cleanupMu.Unlock()
	return a.store.Cleanup(), nil
}
