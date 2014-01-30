package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"path/filepath"
	"sync"
	"time"

	"github.com/goraft/raft"
	"github.com/gorilla/mux"
	"stripe-ctf.com/sqlcluster/log"
	"stripe-ctf.com/sqlcluster/sql"
	"stripe-ctf.com/sqlcluster/transport"
	"stripe-ctf.com/sqlcluster/util"
)

type Server struct {
	name       string
	path       string
	listen     string
	router     *mux.Router
	raftServer raft.Server
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
		router: mux.NewRouter(),
		client: &http.Client{Transport: &http.Transport{Dial: transport.UnixDialer}},
	}

	log.Println(s.name)

	return s, nil
}

// Starts the server.
func (s *Server) ListenAndServe(leader string) error {
	var err error

	cs, err := transport.Encode(s.listen)
	if err != nil {
		return err
	}

	log.Printf("Initializing Raft Server: %s", s.path)

	// Initialize and start Raft server.
	transporter := raft.NewHTTPTransporter("/raft")
	tmp, _ := s.client.Transport.(*http.Transport)
	*transporter.Transport = *tmp
	s.raftServer, err = raft.NewServer(s.name, s.path, transporter, nil, s.sql, "")
	s.raftServer.SetElectionTimeout(150 * time.Millisecond)
	s.raftServer.SetHeartbeatTimeout(50 * time.Millisecond)
	if err != nil {
		log.Fatal(err)
	}
	transporter.Install(s.raftServer, s)
	s.raftServer.Start()

	if leader != "" {
		// Join to leader if specified.

		log.Println("Attempting to join leader:", leader)

		if !s.raftServer.IsLogEmpty() {
			log.Fatal("Cannot join with an existing log")
		}
		if err := s.Join(leader); err != nil {
			log.Fatal(err)
		}
		log.Println("Joined leader:", leader)

	} else if s.raftServer.IsLogEmpty() {
		// Initialize the server by joining itself.

		log.Println("Initializing new cluster")

		_, err := s.raftServer.Do(&raft.DefaultJoinCommand{
			Name:             s.raftServer.Name(),
			ConnectionString: cs,
		})
		if err != nil {
			log.Fatal(err)
		}

	} else {
		log.Println("Recovered from log")
	}

	log.Println("Initializing HTTP server")

	// Initialize and start HTTP server.
	s.httpServer = &http.Server{
		Handler: s.router,
	}

	s.router.HandleFunc("/sql", s.sqlHandler).Methods("POST")
	s.router.HandleFunc("/sql", s.sqlGetHandler).Methods("GET")
	s.router.HandleFunc("/healthcheck", s.healthcheckHandler).Methods("GET")
	s.router.HandleFunc("/join", s.joinHandler).Methods("POST")

	log.Println("Listening at:", cs)

	// Start Unix transport
	l, err := transport.Listen(s.listen)
	if err != nil {
		log.Fatal(err)
	}
	return s.httpServer.Serve(l)
}

func (s *Server) HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	s.router.HandleFunc(pattern, handler)
}

// Join an existing cluster
func (s *Server) Join(primary string) error {
	cs, err := transport.Encode(primary)
	if err != nil {
		return err
	}
	cs2, err := transport.Encode(s.listen)
	if err != nil {
		return err
	}

	command := raft.DefaultJoinCommand{
		Name:             s.raftServer.Name(),
		ConnectionString: cs2,
	}

	for {
		var b bytes.Buffer
		json.NewEncoder(&b).Encode(command)
		log.Println(cs)
		resp, err := s.client.Post(fmt.Sprintf("%s/join", cs), "application/json", &b)
		if err != nil {
			time.Sleep(10 * time.Millisecond)
			continue
		} else {
			resp.Body.Close()
		}

		return nil
	}
}

func (s *Server) joinHandler(w http.ResponseWriter, req *http.Request) {
	command := &raft.DefaultJoinCommand{}

	if err := json.NewDecoder(req.Body).Decode(&command); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_, err := s.raftServer.Do(command)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
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

	// Execute the command against the Raft server.
	if s.raftServer.State() == "leader" {
		log.Printf("Executing query: %#v", query)
		ch := make(chan []byte)
		timeout := make(chan bool)
		go s.doQuery(query, ch)
		go func() {
			time.Sleep(500 * time.Millisecond)
			timeout <- true
			ch <- nil
		}()
		select {
		case response := <-ch:
			if response != nil {
				log.Printf("Returning response to %#v: %#v", query, string(response))
				w.Write(response)
			} else {
				http.Error(w, "Raft error", http.StatusBadRequest)
			}
		case <-timeout:
			http.Error(w, "Timeout", 400)
		}
	} else {
		leader := s.raftServer.Leader()
		if leader == "" {
			peers := s.raftServer.Peers()
			if len(peers) > 0 {
				randi := rand.Int() % len(peers)
				for name, _ := range peers {
					if randi <= 0 {
						leader = name
						break
					}
					randi--
				}
			} else {
				http.Error(w, "No peers", 400)
				return
			}
		}
		host := req.Host[0 : len(req.Host)-18]
		cs := fmt.Sprintf("http://%s%s-.-%s.sock/sql?query=%s", host, leader, leader, url.QueryEscape(query))
		log.Debugf("Redirecting query to %#v: %#v", cs, query)
		http.Redirect(w, req, cs, 302)
	}
}

func (s *Server) sqlGetHandler(w http.ResponseWriter, req *http.Request) {
	query := req.FormValue("query")

	// Execute the command against the Raft server.
	if s.raftServer.State() == "leader" {
		log.Printf("Executing query(f): %#v", query)
		ch := make(chan []byte)
		timeout := make(chan bool)
		go s.doQuery(query, ch)
		go func() {
			time.Sleep(500 * time.Millisecond)
			timeout <- true
			ch <- nil
		}()
		select {
		case response := <-ch:
			if response != nil {
				log.Printf("Returning response to (f) %#v: %#v", query, string(response))
				w.Write(response)
			} else {
				http.Error(w, "Raft error", http.StatusBadRequest)
			}
		case <-timeout:
			http.Error(w, "Timeout", 400)
		}
	} else {
		leader := s.raftServer.Leader()
		if leader == "" {
			time.Sleep(10 * time.Millisecond)
			peers := s.raftServer.Peers()
			if len(peers) > 0 {
				randi := rand.Int() % len(peers)
				for name, _ := range peers {
					if randi <= 0 {
						leader = name
						break
					}
					randi--
				}
			} else {
				http.Error(w, "No peers", 400)
				return
			}
		}
		host := req.Host[0 : len(req.Host)-18]
		cs := fmt.Sprintf("http://%s%s-.-%s.sock/sql?query=%s", host, leader, leader, url.QueryEscape(query))
		log.Debugf("Redirecting query to (f) %#v: %#v", cs, query)
		http.Redirect(w, req, cs, 302)
	}
}

func (s *Server) doQuery(query string, ch chan []byte) {
	resp, err := s.raftServer.Do(NewSqlCommand(query))
	if err != nil {
		log.Println(err)
		ch <- nil
		return
	}
	response, _ := resp.([]byte)
	ch <- response
}

type SqlCommand struct {
	Query string `json:"query"`
}

// Creates a new sql command.
func NewSqlCommand(query string) *SqlCommand {
	return &SqlCommand{
		Query: query,
	}
}

// The name of the command in the log.
func (c *SqlCommand) CommandName() string {
	return "sql"
}

// Executes a query.
func (c *SqlCommand) Apply(server raft.Server) (interface{}, error) {
	sql := server.Context().(*sql.SQL)
	output, err := sql.Execute(c.Query)

	if err != nil {
		var msg string
		if output != nil && len(output.Stderr) > 0 {
			template := `Error executing %#v (%s)

SQLite error: %s`
			msg = fmt.Sprintf(template, c.Query, err.Error(), util.FmtOutput(output.Stderr))
		} else {
			msg = err.Error()
		}

		return nil, errors.New(msg)
	}

	formatted := fmt.Sprintf("SequenceNumber: %d\n%s",
		output.SequenceNumber, output.Stdout)
	return []byte(formatted), nil
}

func (s *Server) healthcheckHandler(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(http.StatusOK)
}
