// APIの提供
package main

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/uryoya/code-executer/sandbox"
)

type server struct {
	mux *http.ServeMux
}

type Req struct {
	Code string `json:"code"`
}

type Rsp struct {
	Status     string        `json:"status"`
	Stdout     string        `json:"stdout"`
	Stderr     string        `json:"stderr"`
	UserTime   time.Duration `json:"user_time"`
	SystemTime time.Duration `json:"system_time"`
}

func newServer() (*server, error) {
	s := &server{mux: http.NewServeMux()}
	s.init()
	return s, nil
}

func (s *server) init() {
	s.mux.HandleFunc("/", s.handleIndex)
	s.mux.HandleFunc("/compile", s.handleCompile)

	staticHandler := http.StripPrefix("/assets/", http.FileServer(http.Dir("./www/assets/")))
	s.mux.Handle("/assets/", staticHandler)
}

func (s *server) handleIndex(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "./www/index.html")
}

func (s *server) handleCompile(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	// json decode
	buf := new(bytes.Buffer)
	io.Copy(buf, r.Body)
	req := Req{}
	json.Unmarshal(buf.Bytes(), &req)

	// compile
	sb, _ := sandbox.Init()
	defer sb.Close()
	sb.AddData("main.go", []byte(req.Code))
	cmd := sb.Command("go", "run", "main.go")
	stdout, _ := cmd.StdoutPipe()
	if err := cmd.Start(); err != nil {
		rsp, _ := json.Marshal(Rsp{Status: "pipe stdout failed"})
		w.Write(rsp)
		return
	}
	stdoutBytes, _ := ioutil.ReadAll(stdout) // cmd.StdoutPipe() は cmd.Wait() 以前に読み切る必要がある
	err := cmd.Wait()
	if err != nil {
		rsp, _ := json.Marshal(Rsp{Status: "cmd wait failed"})
		w.Write(rsp)
		return
	}
	rsp := Rsp{
		Status:     "ok",
		Stdout:     string(stdoutBytes),
		Stderr:     "",
		UserTime:   cmd.ProcessState.UserTime(), // cmd.ProcessState はコマンド実行後に有効になる
		SystemTime: cmd.ProcessState.SystemTime(),
	}
	rspJson, _ := json.Marshal(rsp)
	w.Write(rspJson)
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}
