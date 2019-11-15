package pipeline

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/walter-cd/walter/lib/util"
)

var ConfigFile = "test/pipeline.yml"
var ConfigFileFail = "test/pipeline_fail.yml"
var ConfigFileInterrupt = "test/pipeline_interrupt.yml"

var p, pFail, pInterrupt Pipeline

/******************************************/
/*************** Pass Tests ***************/
/******************************************/

func TestSerial(t *testing.T) {
	err := p.Run("Serial", "0")
	if err != 0 {
		t.Fail()
	}
}

func TestDirectory(t *testing.T) {
	err := p.Run("Directory", "0")
	if err != 0 || p.Stages[1].Tasks[0].Cmd.Dir != "/usr" {
		t.Fail()
	}
}

func TestParallel(t *testing.T) {
	err := p.Run("Parallel", "0")
	if err != 0 {
		t.Fail()
	}
}

func TestParallelSerial(t *testing.T) {
	err := p.Run("Parallel Serial", "0")
	if err != 0 {
		t.Fail()
	}
}

func TestParallelSerialParallel(t *testing.T) {
	err := p.Run("Parallel Serial Parallel", "0")
	if err != 0 {
		t.Fail()
	}
}

func TestScript(t *testing.T) {
	err := p.Run("Script", "0")
	if err != 0 {
		t.Fail()
	}
}

func TestWaitFor(t *testing.T) {
	err := p.Run("Wait For", "0")
	if err != 0 {
		t.Fail()
	}
}

/******************************************/
/*************** Fail Tests ***************/
/******************************************/

func TestCleanFail(t *testing.T) {
	err := pFail.Run("Clean Fail", "0")
	if err == 0 {
		t.Fail()
	}
}

func TestFailedSerial(t *testing.T) {
	err := pFail.Run("Serial Fail", "0")
	if err == 0 {
		t.Fail()
	}
}

func TestFailedDirectory(t *testing.T) {
	err := pFail.Run("Directory Fail", "0")
	if err == 0 {
		t.Fail()
	}
}

func TestFailedParallel(t *testing.T) {
	err := pFail.Run("Parallel Fail", "0")
	if err == 0 {
		t.Fail()
	}
}

func TestFailedParallelSerial(t *testing.T) {
	err := pFail.Run("Parallel Serial Fail", "0")
	if err == 0 {
		t.Fail()
	}
}

func TestFailedParallelSerialParallel(t *testing.T) {
	err := pFail.Run("Parallel Serial Parallel Fail", "0")
	if err == 0 {
		t.Fail()
	}
}

/***********************************************/
/*************** Interrupt Tests ***************/
/***********************************************/

// signalInterrupt find the process of given pid and send a
// SIGINT to the process
func signalInterrupt(t *testing.T, pid int) {
	// Wait 500ms before interrupting
	time.Sleep(500 * time.Millisecond)

	proc, e := os.FindProcess(pid)
	if e != nil {
		t.Fatalf("FATAL: Fail to find current process: %v", e)
	}

	e = proc.Signal(os.Interrupt)
	if e != nil {
		t.Fatalf("FATAL: Fail to interrupt: %v", e)
	}

	// Wait 1s for the cleanup to finish
	time.Sleep(1 * time.Second)
}

// checkInterrupt checks whether the interrupt stopped subsequent
// tasks from running and ran the cleanup stage
func checkInterrupt(t *testing.T, filePrefix string) {
	// norun.tmp is created in the task after the interrupted task
	// so it should not be created at all
	_, err := os.Stat("test/temp/norun.tmp")
	if err == nil {
		t.Error("ERROR: Command executed after interrupt")
	}

	// Cleanup stage should remove the .tmp files with given prefix and an index
	for i := 0; i <= 3; i++ {
		_, err = os.Stat(fmt.Sprintf("test/temp/%s%v.tmp", filePrefix, i))
		if err == nil || !os.IsNotExist(err) {
			t.Error("ERROR: Cleanup not run properly")
		}
	}
}

// checkInterruptTimeout sleeps for an extra 5s before calling checkInterrupt.
// This checks whether SIGKILL is sent if cleanup is not called 5s after interrupt
func checkInterruptTimeout(t *testing.T, filePrefix string) {
	time.Sleep(5 * time.Second)
	checkInterrupt(t, filePrefix)
}

// The following complication is caused by the os.Exit used to end Walter
// when handling interrupts. Test will terminate immediately if os.Exit is
// called within the process, so one completed interrupt of Walter will terminate
// the whole `go test` process.
// Thus runInterruptTask will be called twice for each test case: The first call
// will spawn a subprocess to run current test again with an additional environment
// variable "INTERRUPT_EXIT"; The second call to runInterruptTask, upon receiving
// the value for "INTERRUPT_EXIT", will run Walter and exits, keeping the test
// process intact.
// Reference: https://talks.golang.org/2014/testing.slide#23

// runInterruptTask executes the test in a subprocess and
// returns a channel waiting for the PID of the test process.
func runInterruptTask(t *testing.T, task string, test string) (chan int, chan error) {
	pidChannel := make(chan int, 1)
	cmdChannel := make(chan error, 1)

	if os.Getenv("INTERRUPT_EXIT") == "1" {
		pInterrupt.Run(task, "0")
		return nil, nil
	}

	go func() {
		cmd := exec.Command(os.Args[0], "-test.run="+test)
		cmd.Env = append(os.Environ(), "INTERRUPT_EXIT=1")
		cmd.Stderr = os.Stderr
		e := cmd.Start()
		if e != nil {
			t.Errorf("ERROR: Failed to start command, %v", e)
		}

		pidChannel <- cmd.Process.Pid

		cmdChannel <- cmd.Wait()
	}()

	return pidChannel, cmdChannel
}

func TestInterruptSerial(t *testing.T) {
	pidChannel, cmdChannel := runInterruptTask(t, "Serial Interrupt", "TestInterruptSerial")

	if pidChannel == nil {
		return
	}

	pid := <-pidChannel
	signalInterrupt(t, pid)

	cmdError := <-cmdChannel
	if cmdError != nil {
		t.Errorf("ERROR: Task Failed, %v", cmdError)
	}
	checkInterrupt(t, "TaskSerial")
}

func TestInterruptParallel(t *testing.T) {
	pidChannel, cmdChannel := runInterruptTask(t, "Parallel Interrupt", "TestInterruptParallel")

	if pidChannel == nil {
		return
	}

	pid := <-pidChannel
	signalInterrupt(t, pid)

	cmdError := <-cmdChannel
	if cmdError != nil {
		t.Errorf("ERROR: Task Failed, %v", cmdError)
	}
	checkInterrupt(t, "TaskParallel")
}

func TestInterruptParallelSerial(t *testing.T) {
	pidChannel, cmdChannel := runInterruptTask(t, "Parallel Serial Interrupt", "TestInterruptParallelSerial")

	if pidChannel == nil {
		return
	}

	pid := <-pidChannel
	signalInterrupt(t, pid)

	cmdError := <-cmdChannel
	if cmdError != nil {
		t.Errorf("ERROR: Task Failed, %v", cmdError)
	}
	checkInterrupt(t, "TaskParallelSerial")
}

func TestInterruptParallelSerialParallel(t *testing.T) {
	pidChannel, cmdChannel := runInterruptTask(t, "Parallel Serial Parallel Interrupt", "TestInterruptParallelSerialParallel")

	if pidChannel == nil {
		return
	}

	pid := <-pidChannel
	signalInterrupt(t, pid)

	cmdError := <-cmdChannel
	if cmdError != nil {
		t.Errorf("ERROR: Task Failed, %v", cmdError)
	}
	checkInterrupt(t, "TaskParallelSerialParallel")
}

func TestInterruptGoodScript(t *testing.T) {
	pidChannel, cmdChannel := runInterruptTask(t, "Good Interrupt", "TestInterruptGoodScript")

	if pidChannel == nil {
		return
	}

	pid := <-pidChannel
	signalInterrupt(t, pid)

	cmdError := <-cmdChannel
	if cmdError != nil {
		t.Errorf("ERROR: Task Failed, %v", cmdError)
	}
	checkInterrupt(t, "TaskGood")
}

func TestInterruptTimeout(t *testing.T) {
	pidChannel, cmdChannel := runInterruptTask(t, "Bad Interrupt", "TestInterruptTimeout")

	if pidChannel == nil {
		return
	}

	pid := <-pidChannel
	signalInterrupt(t, pid)

	cmdError := <-cmdChannel
	if cmdError != nil {
		t.Errorf("ERROR: Task Failed, %v", cmdError)
	}
	checkInterruptTimeout(t, "TaskBad")
}

func TestInterruptForce(t *testing.T) {
	pidChannel, cmdChannel := runInterruptTask(t, "Bad Interrupt", "TestInterruptForce")

	if pidChannel == nil {
		return
	}

	pid := <-pidChannel
	// send two interrupts to force kill
	signalInterrupt(t, pid)
	signalInterrupt(t, pid)
	cmdError := <-cmdChannel
	if cmdError != nil {
		t.Errorf("ERROR: Task Failed, %v", cmdError)
	}
	checkInterrupt(t, "TaskBad")
}

func TestInterruptAbort(t *testing.T) {
	pidChannel, cmdChannel := runInterruptTask(t, "Abort Task", "TestInterruptAbort")

	if pidChannel == nil {
		return
	}

	pid := <-pidChannel
	signalInterrupt(t, pid)

	cmdError := <-cmdChannel
	if cmdError != nil {
		t.Errorf("ERROR: Task Failed, %v", cmdError)
	}
	checkInterrupt(t, "TaskAbort")
}

func TestInterruptParallelAbort(t *testing.T) {
	pidChannel, cmdChannel := runInterruptTask(t, "Parallel Abort", "TestInterruptParallelAbort")

	if pidChannel == nil {
		return
	}

	pid := <-pidChannel
	signalInterrupt(t, pid)

	cmdError := <-cmdChannel
	if cmdError != nil {
		t.Errorf("ERROR: Task Failed, %v", cmdError)
	}
	checkInterrupt(t, "TaskAbortParallel")
}

func TestMain(m *testing.M) {
	var err error
	err = os.Chdir(util.Root)
	if err != nil {
		panic("Fail to go to project root directory")
	}
	p, err = LoadFromFile(ConfigFile)
	if err != nil {
		panic("Fail to load configuration file")
	}
	pFail, err = LoadFromFile(ConfigFileFail)
	if err != nil {
		panic("Fail to load configuration file")
	}
	pInterrupt, err = LoadFromFile(ConfigFileInterrupt)
	if err != nil {
		panic("Fail to load configuration file")
	}

	err = os.Mkdir("test/temp", 0777)
	if err != nil && err.(*os.PathError).Err != syscall.EEXIST {
		panic("Fail to create temp directory")
	}

	exitcode := m.Run()

	os.RemoveAll("test/temp")

	os.Exit(exitcode)
}
