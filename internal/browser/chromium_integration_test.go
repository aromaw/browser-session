package browser

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Opt-in, synthetic localhost data only. No CDP, WebDriver or browser automation library.
func TestChromiumStorageIsolation(t *testing.T) {
	exe := os.Getenv("BROWSER_SESSION_TEST_CHROME")
	if exe == "" {
		t.Skip("set BROWSER_SESSION_TEST_CHROME to opt into real Chromium integration")
	}
	type report struct {
		ID     string            `json:"id"`
		Before map[string]string `json:"before"`
		After  map[string]string `json:"after"`
		Error  string            `json:"error"`
	}
	reports := make(chan report, 8)
	var cacheCount atomic.Int32
	var gateCount atomic.Int32
	gate := make(chan struct{})
	mux := http.NewServeMux()
	mux.HandleFunc("/report", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var v report
		if e := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&v); e != nil {
			http.Error(w, e.Error(), 400)
			return
		}
		reports <- v
		w.WriteHeader(204)
	})
	mux.HandleFunc("/barrier", func(w http.ResponseWriter, r *http.Request) {
		if gateCount.Add(1) == 2 {
			close(gate)
		}
		select {
		case <-gate:
			w.WriteHeader(204)
		case <-r.Context().Done():
		}
	})
	mux.HandleFunc("/http-cache", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=3600")
		fmt.Fprint(w, cacheCount.Add(1))
	})
	mux.HandleFunc("/sw.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		token, _ := json.Marshal(r.URL.Query().Get("identity"))
		fmt.Fprintf(w, `self.addEventListener('install', e=>self.skipWaiting());self.addEventListener('activate', e=>e.waitUntil(clients.claim()));self.addEventListener('message', e=>e.ports[0].postMessage(%s));`, token)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Header().Set("Cache-Control", "no-store")
		fmt.Fprint(w, probeHTML)
	})
	server := httptest.NewServer(mux)
	defer server.Close()
	dirs := map[string]string{"a": filepath.Join(t.TempDir(), "profile a"), "b": filepath.Join(t.TempDir(), "profile b")}
	launch := func(id, mode string) error {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		args, e := (Launcher{}).BuildArgs(dirs[id], []string{server.URL + "/?id=" + id + "&mode=" + mode})
		if e != nil {
			return e
		}
		extra := []string{"--headless=new", "--dump-dom", "--virtual-time-budget=10000", "--timeout=30000", "--disable-gpu"}
		if os.Getenv("BROWSER_SESSION_TEST_NO_SANDBOX") == "1" {
			extra = append(extra, "--no-sandbox")
		}
		c := exec.CommandContext(ctx, exe, append(extra, args...)...)
		// Discard output, even in tests. Failures are reported through synthetic probe results.
		return c.Run()
	}
	phase := func(mode string) map[string]report {
		var wg sync.WaitGroup
		errs := make(chan error, 2)
		for _, id := range []string{"a", "b"} {
			wg.Add(1)
			go func(id string) { defer wg.Done(); errs <- launch(id, mode) }(id)
		}
		wg.Wait()
		close(errs)
		for e := range errs {
			if e != nil {
				t.Fatalf("Chromium %s: %v", mode, e)
			}
		}
		got := map[string]report{}
		for range 2 {
			select {
			case r := <-reports:
				if r.Error != "" {
					t.Fatal(r.Error)
				}
				got[r.ID] = r
			case <-time.After(5 * time.Second):
				t.Fatal("browser probe did not finish")
			}
		}
		return got
	}
	writes := phase("write")
	for id, r := range writes {
		for _, key := range []string{"cookie", "local", "session", "idb", "cache", "sw", "opfs"} {
			if r.Before[key] != "" {
				t.Fatalf("fresh %s had %s=%q", id, key, r.Before[key])
			}
			if r.After[key] != id {
				t.Fatalf("write %s %s=%q", id, key, r.After[key])
			}
		}
	}
	if writes["a"].After["http"] == writes["b"].After["http"] {
		t.Fatal("HTTP cache unexpectedly shared")
	}
	reads := phase("read")
	for id, r := range reads {
		for _, key := range []string{"cookie", "local", "idb", "cache", "sw", "opfs"} {
			if r.Before[key] != id {
				t.Fatalf("persistent %s %s=%q", id, key, r.Before[key])
			}
		}
		if r.Before["session"] != "" {
			t.Fatal("new tab should have fresh sessionStorage")
		}
		if r.Before["http"] != writes[id].After["http"] {
			t.Fatal("HTTP cache was not retained in its profile")
		}
	}
}

const probeHTML = `<!doctype html><meta charset="utf-8"><title>Local storage isolation probe</title><script>
(async()=>{
 const q=new URLSearchParams(location.search),id=q.get('id'),mode=q.get('mode');
 const report={id,before:{},after:{},error:''};
 const request=r=>new Promise((resolve,reject)=>{r.onsuccess=()=>resolve(r.result);r.onerror=()=>reject(r.error)});
 try{
  const opening=indexedDB.open('synthetic',1);opening.onupgradeneeded=()=>opening.result.createObjectStore('values');const db=await request(opening);
  const cache=await caches.open('synthetic');const root=await navigator.storage.getDirectory();
  async function read(){
   let file='';try{file=await (await (await root.getFileHandle('identity')).getFile()).text()}catch(e){if(e.name!=='NotFoundError')throw e}
   const reg=await navigator.serviceWorker.getRegistration();let sw='';
   if(reg&&reg.active){sw=await new Promise(resolve=>{const c=new MessageChannel();c.port1.onmessage=e=>resolve(e.data);reg.active.postMessage('identity',[c.port2])})}
   const cached=await cache.match('/synthetic');
   return {cookie:document.cookie.split('; ').find(x=>x.startsWith('identity='))?.slice(9)||'',local:localStorage.getItem('identity')||'',session:sessionStorage.getItem('identity')||'',idb:await request(db.transaction('values').objectStore('values').get('identity'))||'',cache:cached?await cached.text():'',sw,opfs:file,http:await (await fetch('/http-cache')).text()};
  }
  report.before=await read();
  if(mode==='write'){
   await fetch('/barrier');document.cookie='identity='+id+'; Max-Age=3600; SameSite=Lax; Path=/';localStorage.setItem('identity',id);sessionStorage.setItem('identity',id);
   await new Promise((resolve,reject)=>{const tx=db.transaction('values','readwrite');tx.objectStore('values').put(id,'identity');tx.oncomplete=resolve;tx.onerror=()=>reject(tx.error)});
   await cache.put('/synthetic',new Response(id));const f=await root.getFileHandle('identity',{create:true});const stream=await f.createWritable();await stream.write(id);await stream.close();
   await navigator.serviceWorker.register('/sw.js?identity='+id);await navigator.serviceWorker.ready;
   report.after=await read();
  }
  db.close();
 }catch(e){report.error=String(e)}
 await fetch('/report',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(report)});
 document.body.textContent='Complete';
})()
</script><body>Running synthetic localhost checks</body>`

func TestNoAutomationDefaults(t *testing.T) {
	a, e := (Launcher{}).BuildArgs(t.TempDir(), nil)
	if e != nil {
		t.Fatal(e)
	}
	if strings.Contains(strings.Join(a, " "), "debugging") {
		t.Fatal(a)
	}
}
