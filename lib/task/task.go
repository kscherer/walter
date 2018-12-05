package task

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"syscall"

	"golang.org/x/net/context"

	log "github.com/Sirupsen/logrus"
)

const (
	Init = iota
	Running
	Succeeded
	Failed
	Skipped
	Aborted
)

type key int

const BuildID key = 0

type Task struct {
	Name      string
	Command   string
	Directory string
	Env       map[string]string
	Parallel  []*Task
	Serial    []*Task
	Stdout    *bytes.Buffer
	Stderr    *bytes.Buffer
	Status    int
	Cmd       *exec.Cmd
	Include   string
	OnlyIf    string   `yaml:"only_if"`
	WaitFor   *WaitFor `yaml:"wait_for"`
}

type outputHandler struct {
	buf  *bytes.Buffer
	task *Task
}

func (t *Task) Run(ctx context.Context, cancel context.CancelFunc, prevTask *Task) error {
	if t.Command == "" {
		return nil
	}

	if t.Directory != "" {
		re := regexp.MustCompile(`\$[A-Z1-9\-_]+`)
		matches := re.FindAllString(t.Directory, -1)
		for _, m := range matches {
			env := os.Getenv(strings.TrimPrefix(m, "$"))
			t.Directory = strings.Replace(t.Directory, m, env, -1)
		}
	}

	buildID := ctx.Value(BuildID)

	if t.OnlyIf != "" {
		cmd := exec.Command("sh", "-c", t.OnlyIf)
		cmd.Dir = t.Directory

		cmd.Env = os.Environ()
		cmd.Env = append(cmd.Env, fmt.Sprintf("BUILD_ID=%s", buildID))
		for k, v := range t.Env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}

		err := cmd.Run()

		if err != nil {
			log.Warnf("[%s] Skipped because only_if failed: %s", t.Name, err)
			return nil
		}
	}

	if t.WaitFor != nil {
		err := t.wait()
		if err != nil {
			return err
		}
	}

	log.Infof("[%s] Command: %s ", t.Name, t.Command)

	t.Cmd = exec.Command("sh", "-c", t.Command)
	t.Cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	t.Cmd.Dir = t.Directory

	t.Cmd.Env = os.Environ()
	t.Cmd.Env = append(t.Cmd.Env, fmt.Sprintf("BUILD_ID=%s", buildID))
	for k, v := range t.Env {
		t.Cmd.Env = append(t.Cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	if prevTask != nil && prevTask.Stdout != nil {
		t.Cmd.Stdin = bytes.NewBuffer(prevTask.Stdout.Bytes())
	}

	t.Stdout = new(bytes.Buffer)
	t.Stderr = new(bytes.Buffer)

	t.Cmd.Stdout = &outputHandler{t.Stdout, t}
	t.Cmd.Stderr = &outputHandler{t.Stderr, t}

	if err := t.Cmd.Start(); err != nil {
		t.Status = Failed
		return err
	}

	t.Status = Running

	go func(t *Task) {
		for {
			select {
			case <-ctx.Done():
				if t.Status == Running {
					t.Status = Aborted
					t.Cmd.Process.Kill()
					pgid, err := syscall.Getpgid(t.Cmd.Process.Pid)
					if err == nil {
						syscall.Kill(-pgid, syscall.SIGTERM)
					}
					log.Warnf("[%s] aborted", t.Name)
				}
				return
			}
		}
	}(t)

	t.Cmd.Wait()

	if t.Cmd.ProcessState.Success() {
		t.Status = Succeeded
	} else if t.Status == Running {
		t.Status = Failed
		cancel()
		return errors.New("Task failed")
	}

	return nil
}

func (o *outputHandler) Write(b []byte) (n int, err error) {
	if n, err = o.buf.Write(b); err != nil {
		return
	}

	for {
		line, err := o.buf.ReadString('\n')

		if len(line) > 0 {
			if strings.HasSuffix(line, "\n") {
				log.Infof("[%s] %s", o.task.Name, line)
			} else {
				// put back into buffer, it's not a complete line yet
				//  Close() or Flush() have to be used to flush out
				//  the last remaining line if it does not end with a newline
				if _, err := o.buf.WriteString(line); err != nil {
					return 0, err
				}
			}
		}

		if err == io.EOF {
			break
		}

		if err != nil {
			return 0, err
		}
	}

	return len(b), nil
}
