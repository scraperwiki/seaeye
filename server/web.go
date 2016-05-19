package seaeye

import (
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/gorilla/mux"
)

// Server is a http.Server that can gracefully shut down.
// TODO(uwe): Simplify and switch to https://github.com/tylerb/graceful.
type Server struct {
	http.Server
	Listener   net.Listener
	ConnActive int
	ConnMutex  sync.Mutex
}

// Context holds a context for http.FuncHandler.
type Context struct {
	conf   *Config
	db     *ManifestStore
	builds chan *Build
}

// CtxHandlerFunc defines a http.FuncHandler with context.
type CtxHandlerFunc func(ctx *Context, w http.ResponseWriter, req *http.Request)

// NewServer initializes a new HTTP server. The difference to a standard
// net.http server is that it knows about the listener and can stop itself
// gracefully.
func NewServer(conf *Config, db *ManifestStore, builds chan *Build) *Server {
	ctx := &Context{
		conf:   conf,
		db:     db,
		builds: builds,
	}

	router := mux.NewRouter()
	router.Path("/api/{id}").Methods("GET").HandlerFunc(wrap(ctx, getManifestHandler))
	router.Path("/api/{id}").Methods("PUT", "POST").HandlerFunc(wrap(ctx, putManifestHandler))
	router.Path("/api/{id}").Methods("DELETE").HandlerFunc(wrap(ctx, deleteManifestHandler))
	router.Path("/api/{id}/webhook").Methods("PUT", "POST").HandlerFunc(wrap(ctx, runManifestHandler))
	router.Path("/api/{id}/status/{rev}").Methods("GET").HandlerFunc(wrap(ctx, statusHandler))

	srv := &Server{}
	srv.Addr = conf.HostPort
	srv.ConnState = srv.connStateHook()
	srv.Handler = router

	return srv
}

// Start start the http server, listening, accepting, and serving.
func (srv *Server) Start() error {
	ln, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		return err
	}
	srv.Listener = ln

	go func() {
		log.Printf("Error: [web] Socket closed: %v", srv.Serve(ln))
	}()

	return nil
}

// Stop stops the http server, gracefully draining outstanding connections.
func (srv *Server) Stop() error {
	// no new connections should use keep alive
	srv.SetKeepAlivesEnabled(false)

	// Stop listening, therefore accepting
	err := srv.Listener.Close()
	if err != nil {
		return err
	}

	// Drain active connections with grace period
	for t := 100; t > 0 && srv.ConnActive > 0; t-- {
		time.Sleep(100 * time.Millisecond)
	}

	return nil
}

// ConnStateHook is the http.Server ConnState hook called whenever a connection
// changes its state.
func (srv *Server) connStateHook() func(net.Conn, http.ConnState) {
	return func(_ net.Conn, connState http.ConnState) {
		srv.ConnMutex.Lock()
		defer srv.ConnMutex.Unlock()

		switch connState {
		case http.StateNew:
			srv.ConnActive++
		case http.StateHijacked, http.StateClosed:
			srv.ConnActive--
		}
	}
}

func wrap(ctx *Context, handler CtxHandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		handler(ctx, w, req)
	}
}

func getManifestHandler(ctx *Context, w http.ResponseWriter, req *http.Request) {
	id := url.QueryEscape(mux.Vars(req)["id"])

	m, ok := ctx.db.Get(id)
	if !ok {
		http.Error(w, "manifest not found", http.StatusNotFound)
		return
	}

	d, err := yaml.Marshal(&m)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write(d)
}

func putManifestHandler(ctx *Context, w http.ResponseWriter, req *http.Request) {
	id := url.QueryEscape(mux.Vars(req)["id"])

	b, err := ioutil.ReadAll(req.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	m := &Manifest{}
	if err := yaml.Unmarshal(b, m); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Keep a (self-)reference for later use.
	if m.ID == "" {
		m.ID = id
	}

	e, err := m.Parse(ctx.conf)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if _, ok := ctx.db.Get(id); ok {
		http.Error(w, "manifest already exists", http.StatusConflict)
		return
	}

	// Start trigger if configured.
	if e.Trigger != nil {
		hook := func(event *GithubPushEvent) error {
			url := fmt.Sprintf("%s/api/%s/webhook", ctx.conf.HostPort, id)
			g := &GithubTrigger{}
			return g.Post(url, event)
		}
		e.Trigger.Hook = hook

		if err := e.Trigger.Start(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}

	log.Printf("Info: [web] Storing manifest: %s", id)
	ctx.db.Put(id, m)
}

func deleteManifestHandler(ctx *Context, w http.ResponseWriter, req *http.Request) {
	id := url.QueryEscape(mux.Vars(req)["id"])

	m, ok := ctx.db.Get(id)
	if !ok {
		http.Error(w, "manifest not found", http.StatusNotFound)
		return
	}

	e, err := m.Parse(ctx.conf)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := e.Trigger.Stop(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	ctx.db.Put(id, nil)
}

func runManifestHandler(ctx *Context, w http.ResponseWriter, req *http.Request) {
	id := url.QueryEscape(mux.Vars(req)["id"])

	m, ok := ctx.db.Get(id)
	if !ok {
		http.Error(w, "manifest not found", http.StatusNotFound)
		return
	}

	j, err := m.Parse(ctx.conf)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	t := &GithubTrigger{}
	p, err := t.PushEventFromRequest(req)
	if err != nil {
		log.Printf("Error: [web] Failed to parse github Webhook push event: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if p.After == "" || p.Repository.FullName == "" || p.Repository.SSHURL == "" {
		err := fmt.Errorf("missing field(s): after=%s, repository.full_name=%s, repository.ssh_url=%s",
			p.After, p.Repository.FullName, p.Repository.SSHURL)
		log.Printf("Error: [web] Invalid github Webhook push event: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ctx.builds <- &Build{Job: j, Parameter: p}
}

func statusHandler(ctx *Context, w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	id := url.QueryEscape(vars["id"])
	rev := url.QueryEscape(vars["rev"])

	logFilePath := LogFilePath(id, rev)
	http.ServeFile(w, req, logFilePath)
}
