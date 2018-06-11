// APIの提供
package main

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"syscall"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

type server struct {
	docker    *client.Client
	dockerCtx context.Context
	mux       *http.ServeMux
}

type ReqJson struct {
	Code string `json:"code"`
}

type RspJson struct {
	Status     int    `json:"status"`
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	RealTime   int64  `json:"real_time"`
	UserTime   int64  `json:"user_time"`
	SystemTime int64  `json:"system_time"`
}

type RusageJson struct {
	Utime   int64 `json:"utime"`
	Stime   int64 `json:"stime"`
	Maxrss  int64 `json:"maxrss"`
	Minflt  int64 `json:"minflt"`
	Majflt  int64 `json:"majflt"`
	Inblock int64 `json:"inblock"`
	Oublock int64 `json:"oublock"`
	Nvcsw   int64 `json:"nvcsw"`
	Nivcsw  int64 `json:"nivcsw"`
}

type RecordJson struct {
	Stdout string             `json:"stdout"`
	Stderr string             `json:"stderr"`
	Status syscall.WaitStatus `json:"status"`
	Rtime  int64              `json:"rtime"`
	Rusage RusageJson         `json:"rusage"`
}

func panicResponse(w http.ResponseWriter, errMsg string) {
	rsp, _ := json.Marshal(RspJson{Status: errMsg})
	w.Write(rsp)
}

type SourceCodeFile struct {
	Name, Body string
}

func tarMaker(files []SourceCodeFile) (*bytes.Buffer, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for _, file := range files {
		header := &tar.Header{
			Name: file.Name,
			Mode: 0600,
			Size: int64(len(file.Body)),
		}
		if err := tw.WriteHeader(header); err != nil {
			return nil, errors.New("tw.WriteHeader({" + header.Name + "}): " + err.Error())
		}
		if _, err := tw.Write([]byte(file.Body)); err != nil {
			return nil, errors.New("tw.Write({" + header.Name + "}): " + err.Error())
		}
	}
	if err := tw.Close(); err != nil {
		return nil, errors.New("tar writer close failed.")
	}
	return &buf, nil
}

func newServer() (*server, error) {
	ctx := context.Background()
	cli, err := client.NewEnvClient()
	if err != nil {
		return nil, err // write alternative error
	}
	s := &server{
		docker:    cli,
		dockerCtx: ctx,
		mux:       http.NewServeMux(),
	}
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

// Dockerを使ったSandbox環境を使ってコンパイルと実行をする
func (s *server) handleCompile(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	// json decode
	buf := new(bytes.Buffer)
	io.Copy(buf, r.Body)
	req := ReqJson{}
	json.Unmarshal(buf.Bytes(), &req)

	// compile and run
	containerConf := container.Config{
		Image:      "golang-playground",
		WorkingDir: "/app",
		Cmd:        []string{"record", "go", "run", "main.go"},
		Tty:        true,
	}
	runRsp, err := s.docker.ContainerCreate(
		s.dockerCtx,
		&containerConf,
		nil,
		nil,
		"")
	if err != nil {
		rsp, _ := json.Marshal(RspJson{Status: err.Error()})
		w.Write(rsp)
		return
	}

	srcCode, err := tarMaker([]SourceCodeFile{{"main.go", req.Code}})
	if err != nil {
		panicResponse(w, "make tar: "+err.Error())
		return
	}
	if err := s.docker.CopyToContainer(s.dockerCtx, runRsp.ID, containerConf.WorkingDir, srcCode,
		types.CopyToContainerOptions{}); err != nil {
		if err != nil {
			panicResponse(w, "copy to container: "+err.Error())
			return
		}
	}

	if err := s.docker.ContainerStart(s.dockerCtx, runRsp.ID, types.ContainerStartOptions{}); err != nil {
		if err != nil {
			panicResponse(w, "container start: "+err.Error())
			return
		}
	}

	statusCh, errCh := s.docker.ContainerWait(s.dockerCtx, runRsp.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			panicResponse(w, "container wait: "+err.Error())
			return
		}
	case <-statusCh:
	}

	out, err := s.docker.ContainerLogs(s.dockerCtx, runRsp.ID,
		types.ContainerLogsOptions{ShowStdout: true, ShowStderr: true})
	if err != nil {
		panicResponse(w, "container log: "+err.Error())
		return
	}
	stdoutBytes, _ := ioutil.ReadAll(out)
	record := RecordJson{}
	json.Unmarshal(stdoutBytes, &record)
	rspJson := RspJson{
		Status:     record.Status.ExitStatus(),
		Stdout:     record.Stdout,
		Stderr:     record.Stderr,
		RealTime:   record.Rtime,
		UserTime:   record.Rusage.Utime,
		SystemTime: record.Rusage.Stime,
	}
	rsp, _ := json.Marshal(rspJson)
	w.Write(rsp)
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}
