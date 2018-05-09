// sandboxパッケージは簡易的なサンドボックスを提供します。
// TODO: chrootなどを使ってシステムの利用を制限させる
// TODO: chrootで使われるイメージを決定する

package sandbox

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
)

// Sandboxはサンドボックス環境のメタデータを格納します。
type Sandbox struct {
	Root    string // chroot の起点となるディレクトリ
	WorkDir string // Command()で設定される基本的なワーキングディレクトリ
}

// InitはSandboxを初期化します。
// TODO: chrootで使われるイメージを指定できるようにする
func Init() (*Sandbox, error) {
	dir, err := ioutil.TempDir("", "executor-sandbox")
	if err != nil {
		return nil, fmt.Errorf("Sandboxは一時ディレクトリの作成に失敗しました")
	}
	return &Sandbox{dir, dir}, nil
}

// CommandはSandbox上で実行されるCmdを生成します。
// TODO: chroot環境で実行する
func (s *Sandbox) Command(name string, arg ...string) *exec.Cmd {
	//arg = append([]string{s.Root, name}, arg...)
	//return exec.Command("chroot", arg...)
	cmd := exec.Command(name, arg...)
	cmd.Dir = s.WorkDir
	return cmd
}

// AddFileはSandbox上にファイルを展開します。
func (s *Sandbox) AddFile(dst, src string) error {
	dstFp, err := os.Create(filepath.Join(s.Root, dst))
	if err != nil {
		return err
	}
	defer dstFp.Close()
	srcFp, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFp.Close()
	_, err = io.Copy(dstFp, srcFp)
	if err != nil {
		return err
	}
	return nil
}

// AddDataはSandbox上にバイト列のデータをファイルとして書き込みます。
func (s *Sandbox) AddData(dst string, src []byte) error {
	dstFp, err := os.Create(filepath.Join(s.Root, dst))
	if err != nil {
		return err
	}
	defer dstFp.Close()
	_, err = dstFp.Write(src)
	if err != nil {
		return err
	}
	return nil
}

// Closeは使い終わったSandboxをホストOSから削除します。
// Sandboxを生成した後は必ずdeferで呼び出すようにしてください。
func (s *Sandbox) Close() {
	os.RemoveAll(s.Root)
}
