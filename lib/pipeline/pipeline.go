package pipeline

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strings"
	"sync"

	log "github.com/Sirupsen/logrus"

	"github.com/go-yaml/yaml"
	"github.com/walter-cd/walter/lib/task"
	"github.com/walter-cd/walter/lib/util"
)

// Pipeline holds global settings and an array stages
type Pipeline struct {
	Stages []Stage
}

// Stage has a name and arrays of tasks and cleanup tasks
type Stage struct {
	Name    string
	Tasks   Tasks
	Cleanup Tasks
}

// Tasks typedef for array of tasks
type Tasks []*task.Task

// Handling function for interrupts
var handler util.InterruptHandler

// Load the yaml file into the Pipeline object
func Load(b []byte) (Pipeline, error) {
	p := Pipeline{}
	err := yaml.Unmarshal(b, &p)
	if err != nil {
		log.Error(err)
	}

	return p, nil
}

// LoadFromFile returns a pipeline object
func LoadFromFile(file string) (Pipeline, error) {
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return Pipeline{}, err
	}
	return Load(data)
}

// handleInterrupt sets up the channel for listening system interrupt signals
func handleInterrupt() {
	log.Warningln("Interrupted, aborting all running tasks...")
	log.Warningln("Press Ctrl-c again to force quit")

	failed := handler()
	if failed {
		os.Exit(1)
	}

	os.Exit(0)
}

// Run all the tasks and cleanup commands in the pipeline
func (p *Pipeline) Run(stageToRun string, buildID string) int {
	// Setup interrupt handling
	util.PrepareInterrupt(handleInterrupt)

	failed := false

	numStages := len(p.Stages)
	for i, stage := range p.Stages {
		name := stage.Name
		numStage := i + 1
		if stageToRun != "" && name != stageToRun {
			log.Info(fmt.Sprintf("Stage %s [%d of %d] skipped", name, numStage, numStages))
			continue
		}
		log.Info(fmt.Sprintf("Stage %s [%d of %d] started", name, numStage, numStages))
		ctx, cancel := context.WithCancel(context.WithValue(context.Background(), task.BuildID, buildID))

		// Update function to handle interrupt for current stage
		handler = func() bool {
			// Cancel current task
			cancel()

			// Wait for all tasks to cancel
			task.CancelWg.Wait()

			// Print skip messages for all subsequent stages if no specific stage is set to run
			if stageToRun == "" {
				for j, stage := range p.Stages[i+1:] {
					log.Warnf("Stage %s [%d of %d] skipped: interrupt received", stage.Name, i+j+2, numStages)
				}
			}

			select {
			// If user signals a force quit, skip cleanup
			case <-util.ForceQuit():
				log.Warningln("Received second Ctrl-c, force quit...")
				return true
			// Else run cleanup for current stage
			default:
				log.Info("Cleaning up...")
				return p.cleanupStage(buildID, stage, numStage)
			}
		}

		err := p.runTasks(ctx, cancel, stage.Tasks, nil)

		if util.Interrupted(ctx) {
			// This line effectively blocks subsequent lines from running
			// if an interruption occurs when the task is running. A call
			// to exit the program will happen in handleInterrupt function
			util.WaitClean()
		}

		if err != nil {
			log.Error(fmt.Sprintf("Stage %s failed", name))
			failed = true
		} else {
			log.Info(fmt.Sprintf("Stage %s succeeded", name))
		}

		cleanFailed := p.cleanupStage(buildID, stage, numStage)

		if failed || cleanFailed {
			return 1
		}
	}

	return 0
}

// cleanupStage runs the cleanup task for the stage specified
func (p *Pipeline) cleanupStage(buildID string, stage Stage, numStage int) bool {
	failed := false
	name := stage.Name
	numStages := len(p.Stages)

	if len(stage.Cleanup) > 0 {
		log.Info(fmt.Sprintf("Stage %s [%d of %d] cleanup started", name, numStage, numStages))
		ctx, cancel := context.WithCancel(context.WithValue(context.Background(), task.BuildID, buildID))
		err := p.runTasks(ctx, cancel, stage.Cleanup, nil)
		if err != nil {
			log.Error(fmt.Sprintf("Stage %s cleanup failed", name))
			failed = true
		} else {
			log.Info(fmt.Sprintf("Stage %s cleanup succeeded", name))
		}
	}

	return failed
}

func includeTasks(file string) (Tasks, error) {
	re := regexp.MustCompile(`\$[A-Z1-9\-_]+`)
	matches := re.FindAllString(file, -1)
	for _, m := range matches {
		env := os.Getenv(strings.TrimPrefix(m, "$"))
		file = strings.Replace(file, m, env, -1)
	}

	data, err := ioutil.ReadFile(file)
	tasks := Tasks{}
	if err != nil {
		return tasks, err
	}

	err = yaml.Unmarshal(data, &tasks)
	if err != nil {
		return tasks, err
	}

	return tasks, err
}

func (p *Pipeline) runTasks(ctx context.Context, cancel context.CancelFunc, tasks Tasks, prevTask *task.Task) error {
	failed := false

	// Add first level tasks into the wait group
	taskNum := len(tasks)
	task.CancelWg.Add(taskNum)

	defer task.CancelWg.Add(-taskNum)

	for i, t := range tasks {
		if util.Interrupted(ctx) {
			// Skip execution of all subsequent tasks if interrupted
			log.Warnf("[%s] Task skipped: interrupt received", t.Name)
			continue
		}

		if i > 0 {
			prevTask = tasks[i-1]
		}

		if t.Include != "" {
			include, err := includeTasks(t.Include)
			if err != nil {
				log.Error(err)
				return err
			}

			err = p.runTasks(ctx, cancel, include, prevTask)
			if err != nil {
				failed = true
			}

			continue
		}

		if len(t.Parallel) > 0 {
			err := p.runParallel(ctx, cancel, t, prevTask)
			if err != nil {
				failed = true
			}
			continue
		}

		if len(t.Serial) > 0 {
			err := p.runSerial(ctx, cancel, t, prevTask)
			if err != nil {
				failed = true
			}
			continue
		}

		if failed || (i > 0 && tasks[i-1].Status == task.Failed) {
			t.Status = task.Skipped
			failed = true
			log.Warnf("[%s] Task skipped because previous task failed", t.Name)
			continue
		}

		err := t.Run(ctx, cancel, prevTask)
		if err != nil {
			failed = true
			log.Errorf("[%s] %s", t.Name, err)
		}

	}

	if failed {
		return errors.New("One of the tasks failed")
	}

	return nil
}

func (p *Pipeline) runParallel(ctx context.Context, cancel context.CancelFunc, t *task.Task, prevTask *task.Task) error {

	var tasks Tasks
	for _, child := range t.Parallel {
		if child.Include != "" {
			include, err := includeTasks(child.Include)
			if err != nil {
				log.Error(err)
				return err
			}
			tasks = append(tasks, include...)
		} else {
			tasks = append(tasks, child)
		}
	}

	log.Infof("[%s] Start task", t.Name)

	var wg sync.WaitGroup
	for _, t := range tasks {
		wg.Add(1)
		go func(t *task.Task) {
			defer wg.Done()

			if len(t.Serial) > 0 {
				err := p.runSerial(ctx, cancel, t, prevTask)
				if err != nil {
					log.Errorf("[%s] %v", t.Name, err)
				}
				return
			}

			err := t.Run(ctx, cancel, prevTask)
			if err != nil {
				log.Errorf("[%s] %s", t.Name, err)
			}

		}(t)
	}
	wg.Wait()

	// If an interrupt is received, return immediately
	if util.Interrupted(ctx) {
		return nil
	}

	t.Status = task.Succeeded

	t.Stdout = new(bytes.Buffer)
	t.Stderr = new(bytes.Buffer)

	for _, child := range tasks {
		t.Stdout.Write(child.Stdout.Bytes())
		t.Stderr.Write(child.Stderr.Bytes())
		if child.Status == task.Failed {
			t.Status = task.Failed
		}
	}

	log.Infof("[%s] End task", t.Name)
	if t.Status == task.Failed {
		return errors.New("Parallel task failed")
	}

	return nil
}

func (p *Pipeline) runSerial(ctx context.Context, cancel context.CancelFunc, t *task.Task, prevTask *task.Task) error {
	var tasks Tasks
	for _, child := range t.Serial {
		if child.Include != "" {
			include, err := includeTasks(child.Include)
			if err != nil {
				log.Error(err)
			}
			tasks = append(tasks, include...)
		} else {
			tasks = append(tasks, child)
		}
	}

	log.Infof("[%s] Start task", t.Name)

	err := p.runTasks(ctx, cancel, tasks, prevTask)
	if err != nil {
		log.Errorf("[%s] %s", t.Name, err)
	}

	// If an interrupt is received, return immediately
	if util.Interrupted(ctx) {
		return nil
	}

	t.Status = task.Succeeded
	for _, child := range tasks {
		if child.Status == task.Failed {
			t.Status = task.Failed
		}
	}

	t.Stdout = new(bytes.Buffer)
	t.Stderr = new(bytes.Buffer)

	lastTask := tasks[len(tasks)-1]

	// Stdout and Stderr buffer of lastTask could be nil if it is skipped
	if lastTask.Stdout != nil {
		t.Stdout.Write(lastTask.Stdout.Bytes())
		t.Stderr.Write(lastTask.Stderr.Bytes())
	}

	log.Infof("[%s] End task", t.Name)
	if t.Status == task.Failed {
		return errors.New("Serial task failed")
	}

	return nil
}
