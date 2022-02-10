package diffutil

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

type Differ struct {
	basedir string
}

func NewDiffer() (*Differ, error) {
	basedir, err := os.MkdirTemp("", "diff-*")
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(filepath.Join(basedir, "old"), 0700); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(basedir, "new"), 0700); err != nil {
		return nil, err
	}

	return &Differ{
		basedir: basedir,
	}, nil
}

func (differ *Differ) AddOld(name string, data []byte) error {
	oldDir, _ := differ.subdirs()
	return os.WriteFile(filepath.Join(oldDir, name), data, 0600)
}

func (differ *Differ) AddNew(name string, data []byte) error {
	_, newDir := differ.subdirs()
	return os.WriteFile(filepath.Join(newDir, name), data, 0600)
}

func (differ *Differ) Run(command string, stdout io.Writer, stderr io.Writer) error {
	shell := ""
	var args []string
	if runtime.GOOS == "windows" {
		shell = "cmd"
		args = append(args, "/C")
	} else {
		shell = "/bin/sh"
		args = append(args, "-c")
	}
	if envShell := os.Getenv("SHELL"); envShell != "" {
		shell = envShell
	}

	oldDir, newDir := differ.subdirs()
	args = append(args, fmt.Sprintf("%s %s %s", command, oldDir, newDir))

	cmd := exec.Command(shell, args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	return cmd.Run()
}

func (differ *Differ) Cleanup() error {
	return os.RemoveAll(differ.basedir)
}

func (differ *Differ) subdirs() (string, string) {
	return filepath.Join(differ.basedir, "old"), filepath.Join(differ.basedir, "new")
}
