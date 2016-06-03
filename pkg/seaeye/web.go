// TODO(uwe): Simplify and switch to gin+graceful

package seaeye

import (
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
)

// Server is a http.Server that can gracefully shut down.
type Server struct {
	http.Server
	Listener   net.Listener
	ConnActive int
	ConnMutex  sync.Mutex
}

// ServerState provides a global context state for http.FuncHandler.
type ServerState struct {
	config *Config
	builds chan *Build
	stats  func() Stats
}

// StateHandlerFunc defines a http.FuncHandler with state.
type StateHandlerFunc func(state *ServerState, w http.ResponseWriter, req *http.Request)

type httpError struct {
	error
	Status int
}

// NewWebServer initializes a new HTTP server. The difference to a standard
// net.http server is that it knows about the listener and can stop itself
// gracefully.
func NewWebServer(conf *Config, builds chan *Build, stats func() Stats) *Server {
	state := &ServerState{
		config: conf,
		builds: builds,
		stats:  stats,
	}

	router := mux.NewRouter()
	router.Path("/").Methods("GET").HandlerFunc(wrap(state, indexHandler))
	router.Path("/health").Methods("GET").HandlerFunc(wrap(state, healthHandler))
	router.Path("/jobs/{id}/status/{rev}").Methods("GET").HandlerFunc(wrap(state, statusJobHandler))
	router.Path("/login").Methods("GET").HandlerFunc(wrap(state, loginHandler))
	router.Path("/webhook").Methods("PUT", "POST").HandlerFunc(wrap(state, webhookHandler))

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
		log.Printf("[E][web] Socket closed: %v", srv.Serve(ln))
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

	// Drain active connections with grace period of 10s.
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

func wrap(state *ServerState, handler StateHandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		handler(state, w, req)
	}
}

func indexHandler(state *ServerState, w http.ResponseWriter, req *http.Request) {
	// TODO(uwe): implement
}

func healthHandler(state *ServerState, w http.ResponseWriter, req *http.Request) {
	stats := state.stats()
	for k, v := range stats {
		fmt.Fprintf(w, "%s=%v\n", k, v)
	}
}

func loginHandler(state *ServerState, w http.ResponseWriter, req *http.Request) {
	// TODO(uwe): implement
}

func webhookHandler(state *ServerState, w http.ResponseWriter, req *http.Request) {
	s, err := sourceFromRequest(req)
	if err != nil {
		log.Printf("[E][web] Invalid github webhook push event: %v", err)
		msg, code := toHTTPError(err)
		http.Error(w, msg, code)
		return
	}

	j := &Job{Config: state.config}
	state.builds <- &Build{Job: j, Source: s}
}

func statusJobHandler(state *ServerState, w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	id := vars["id"]
	rev := vars["rev"]

	logFilePath, err := state.config.LogFilePath(id, rev)
	if err != nil {
		msg, code := toHTTPError(err)
		http.Error(w, msg, code)
		return
	}

	// TODO(uwe): Stream output

	b, err := ioutil.ReadFile(logFilePath)
	if err != nil {
		msg, code := toHTTPError(err)
		http.Error(w, msg, code)
		return
	}

	w.Header().Set("Content-type", "text/html")
	w.Write(toHTML(b))
}

func sourceFromRequest(req *http.Request) (*Source, error) {
	e, err := PushEventFromRequest(req)
	if err != nil {
		err := fmt.Errorf("failed to parse: %v", err)
		return nil, &httpError{error: err, Status: http.StatusBadRequest}
	}

	if *e.After == "" || *e.Repo.FullName == "" || *e.Repo.URL == "" {
		err := fmt.Errorf("missing field(s): after=%s, repository.full_name=%s, repository.ssh_url=%s",
			*e.After, *e.Repo.FullName, *e.Repo.URL)
		return nil, &httpError{error: err, Status: http.StatusBadRequest}
	}

	parts := strings.Split(*e.Repo.FullName, "/")
	if parts == nil || len(parts) != 2 {
		err := fmt.Errorf("invalid owner/repo %s", *e.Repo.FullName)
		return nil, &httpError{error: err, Status: http.StatusBadRequest}
	}

	s := &Source{
		Owner: parts[0],
		Repo:  parts[1],
		Rev:   *e.After,
		URL:   *e.Repo.URL,
	}
	return s, nil
}

func toHTTPError(err error) (msg string, httpStatus int) {
	if httpErr, ok := err.(*httpError); ok {
		return httpErr.Error(), httpErr.Status
	}
	if os.IsNotExist(err) {
		return fmt.Sprintf("%d %s", http.StatusNotFound,
			http.StatusText(http.StatusNotFound)), http.StatusNotFound
	}
	if os.IsPermission(err) {
		return fmt.Sprintf("%d %s", http.StatusForbidden,
			http.StatusText(http.StatusForbidden)), http.StatusForbidden
	}
	return err.Error(), http.StatusInternalServerError
}
