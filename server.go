// APIの提供
package main

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"time"
	"github.com/kait-takanolab/code-executer/sandbox"
	"github.com/docker/docker/client"
	"golang.org/x/net/context"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types"
	"archive/tar"
	"github.com/pkg/errors"
)

type server struct {
	docker *client.Client
	dockerCtx context.Context
	mux *http.ServeMux
}

type ReqJson struct {
	Code string `json:"code"`
}

type RspJson struct {
	Status     string        `json:"status"`
	Stdout     string        `json:"stdout"`
	Stderr     string        `json:"stderr"`
	UserTime   time.Duration `json:"user_time"`
	SystemTime time.Duration `json:"system_time"`
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
		docker: cli,
		dockerCtx: ctx,
		mux: http.NewServeMux(),
	}
	s.init()
	return s, nil
}

func (s *server) init() {
	s.mux.HandleFunc("/", s.handleIndex)
	s.mux.HandleFunc("/compile", s.handleDockerCompile)

	staticHandler := http.StripPrefix("/assets/", http.FileServer(http.Dir("./www/assets/")))
	s.mux.Handle("/assets/", staticHandler)
}

func (s *server) handleIndex(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "./www/index.html")
}

// 自作のSandbox環境を使ってコンパイルと実行をする
func (s *server) handleCompile(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	// json decode
	buf := new(bytes.Buffer)
	io.Copy(buf, r.Body)
	req := ReqJson{}
	json.Unmarshal(buf.Bytes(), &req)

	// compile and run
	sb, _ := sandbox.Init()
	defer sb.Close()
	sb.AddData("main.go", []byte(req.Code))
	cmd := sb.Command("go", "run", "main.go")
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()
	if err := cmd.Start(); err != nil {
		rsp, _ := json.Marshal(RspJson{Status: "pipe stdout failed"})
		w.Write(rsp)
		return
	}
	stdoutBytes, _ := ioutil.ReadAll(stdout) // cmd.StdoutPipe() は cmd.Wait() 以前に読み切る必要がある
	stderrBytes, _ := ioutil.ReadAll(stderr)
	err := cmd.Wait()
	status := "ok"
	if err != nil {
		status = "cmd wait failed"
	}
	rspJson := RspJson{
		Status:     status,
		Stdout:     string(stdoutBytes),
		Stderr:     string(stderrBytes),
		UserTime:   cmd.ProcessState.UserTime(), // cmd.ProcessState はコマンド実行後に有効になる
		SystemTime: cmd.ProcessState.SystemTime(),
	}
	rsp, _ := json.Marshal(rspJson)
	w.Write(rsp)
}

// Dockerを使ったSandbox環境を使ってコンパイルと実行をする
func (s *server) handleDockerCompile(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	// json decode
	buf := new(bytes.Buffer)
	io.Copy(buf, r.Body)
	req := ReqJson{}
	json.Unmarshal(buf.Bytes(), &req)

	// compile and run
	containerConf := container.Config{
		Image: "golang-playground",
		WorkingDir: "/app",
		Cmd:  []string{"record", "go", "run", "main.go"},
		Tty: true,
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
		panicResponse(w, "make tar: " + err.Error())
		return
	}
	if err := s.docker.CopyToContainer(s.dockerCtx, runRsp.ID, containerConf.WorkingDir, srcCode,
		types.CopyToContainerOptions{}); err != nil {
		if err != nil {
			panicResponse(w, "copy to container: " + err.Error())
			return
		}
	}


	if err := s.docker.ContainerStart(s.dockerCtx, runRsp.ID, types.ContainerStartOptions{}); err != nil {
		if err != nil {
			panicResponse(w, "container start: " + err.Error())
			return
		}
	}

	statusCh, errCh := s.docker.ContainerWait(s.dockerCtx, runRsp.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			panicResponse(w, "container wait: " + err.Error())
			return
		}
		case <-statusCh:
	}

	out, err := s.docker.ContainerLogs(s.dockerCtx, runRsp.ID,
		types.ContainerLogsOptions{ShowStdout: true, ShowStderr: true})
	if err != nil {
		panicResponse(w, "container log: " + err.Error())
		return
	}
	stdoutBytes, _ := ioutil.ReadAll(out)
	rspJson := RspJson{Status: "ok", Stdout: string(stdoutBytes)}
	rsp, _ := json.Marshal(rspJson)
	w.Write(rsp)
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}
