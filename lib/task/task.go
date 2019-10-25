package task

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/walter-cd/walter/lib/util"
)

const (
	Init = iota
	Running
	Succeeded
	Failed
	Skipped
	Aborted
)

const timeoutBeforeKill = 5 * time.Second

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

var CancelWg sync.WaitGroup

var statusLock sync.Mutex

func (t *Task) Run(ctx context.Context, cancel context.CancelFunc, prevTask *Task) error {
	if t.Command == "" {
		return nil
	}

	// Add current task into the wait group
	CancelWg.Add(1)

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

	// Channel to receive the return value or signal from cmd.Wait()
	commandFinished := make(chan error, 1)

	// Channel to inform the goroutine below that current task has quit
	quitTask := make(chan struct{}, 1)

	go func(t *Task) {
		defer CancelWg.Done()
		for {
			select {
			case <-ctx.Done(): // Handle interrupt or failures and quit
				// Acquire lock to read and change task status
				statusLock.Lock()
				defer statusLock.Unlock()

				if t.Status == Running {
					t.Status = Aborted

					if _, err := os.FindProcess(t.Cmd.Process.Pid); err == nil {
						syscall.Kill(-t.Cmd.Process.Pid, syscall.SIGTERM)

						select {
						case <-time.After(timeoutBeforeKill):
							log.Warnf("[%s] Did not terminate in %v, sending SIGKILL...", t.Name, timeoutBeforeKill)
							err := syscall.Kill(-t.Cmd.Process.Pid, syscall.SIGKILL)
							if err != nil {
								log.Errorf("[%s] failed to terminate: %v", t.Name, err)
							}
						case err := <-commandFinished:
							// https://github.com/golang/go/issues/19798
							// Go does not mark process as commandFinished when interrupted by signal, so the signal in the
							// error returned by Cmd.Wait() is first extracted and then compared with SIGTERM signal
							// provided by the OS
							if err != nil {
								exitSig := err.(*exec.ExitError).Sys().(syscall.WaitStatus).Signal()

								if exitSig != syscall.SIGTERM && exitSig != syscall.SIGINT {
									log.Errorf("[%s] commandFinished with error: %v", t.Name, err)
								}
							}
						}

						log.Warnf("[%s] aborted", t.Name)
					}
				}
				return
			case <-quitTask: // Quit after current task finished
				return
			}
		}
	}(t)

	commandFinished <- t.Cmd.Wait()

	// Acquire lock to read and change task status
	statusLock.Lock()
	defer statusLock.Unlock()

	// Flush any remaining bytes on the buffer
	var p []byte
	if _, err := t.Stderr.Read(p); len(p) > 0 && err == nil {
		log.Infof("[%s] %s", t.Name, string(p))
	}
	if _, err := t.Stdout.Read(p); len(p) > 0 && err == nil {
		log.Infof("[%s] %s", t.Name, string(p))
	}

	// If the current task is interrupted, abort changing the status of
	// the task and return immediately
	if util.Interrupted(ctx) {
		return nil
	}

	if t.Cmd.ProcessState.Success() {
		t.Status = Succeeded
	} else if t.Status == Running {
		t.Status = Failed
		cancel()

		close(quitTask)
		return errors.New("Task failed")
	}

	close(quitTask)
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
