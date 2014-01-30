package server

import (
	"fmt"
	"io/ioutil"
	"net/http"
	_ "net/http/pprof"
	"net/url"
	"path/filepath"
	"sync"

	"strconv"
	"stripe-ctf.com/sqlcluster/log"
	"stripe-ctf.com/sqlcluster/sql"
	"stripe-ctf.com/sqlcluster/transport"
	"stripe-ctf.com/sqlcluster/util"
)

type Server struct {
	name       string
	path       string
	listen     string
	peers      []string
	httpServer *http.Server
	sql        *sql.SQL
	client     *http.Client
	mutex      sync.RWMutex
}

// Creates a new server.
func New(path, listen string) (*Server, error) {
	sqlPath := filepath.Join(path, "storage.sql")
	util.EnsureAbsent(sqlPath)

	s := &Server{
		name:   listen[2:7],
		path:   path,
		listen: listen,
		sql:    sql.NewSQL(sqlPath),
		client: &http.Client{Transport: &http.Transport{Dial: transport.UnixDialer}},
	}

	peers := make([]string, 4)

	j := 0
	for i := 0; i < 5; i++ {
		name := fmt.Sprintf("node%d", i)
		if name != s.name {
			peers[j] = name
			j++
		}
	}
	s.peers = peers

	log.Println(s.name)

	return s, nil
}

// Starts the server.
func (s *Server) ListenAndServe(leader string) error {
	var err error

	log.Println("Initializing HTTP server")

	// Initialize and start HTTP server.
	s.httpServer = &http.Server{
		Handler: http.HandlerFunc(s.handleRouting),
	}

	// Start Unix transport
	l, err := transport.Listen(s.listen)
	if err != nil {
		log.Fatal(err)
	}
	return s.httpServer.Serve(l)
}

func (s *Server) handleRouting(w http.ResponseWriter, req *http.Request) {
	if req.Method == "GET" {
		s.sqlGetHandler(w, req)
	} else { // POST
		s.sqlHandler(w, req)
	}
}

// This is the only user-facing function, and accordingly the body is
// a raw string rather than JSON.
func (s *Server) sqlHandler(w http.ResponseWriter, req *http.Request) {
	// Read the value from the POST body.
	q, err := ioutil.ReadAll(req.Body)
	if err != nil {
		log.Printf("Couldn't read body: %s", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
	}
	query := string(q)

	if s.name == "node0" {
		w.Write(s.Execute(query))
	} else {
		host := req.Host[0 : len(req.Host)-18]
		// cs := fmt.Sprintf("http://%snode0-.-node0.sock/sql?query=%s", host, url.QueryEscape(query))
		//log.Debugf("Redirecting to node0: %s", s.name)
		http.Redirect(w, req, "http://"+host+"node0-.-node0.sock/sql?query="+url.QueryEscape(query), 302)
	}
}

func (s *Server) sqlGetHandler(w http.ResponseWriter, req *http.Request) {
	w.Write(s.Execute(req.FormValue("query")))
}

// Executes a query.
func (s *Server) Execute(query string) []byte {
	output, sequence := s.sql.Execute(query)

	//formatted := fmt.Sprintf("SequenceNumber: %d\n%s", sequence, output)
	return []byte("SequenceNumber: " + strconv.Itoa(sequence) + "\n" + output)
}
