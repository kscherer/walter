Walter
======

## Build

```bash
# Get walter from source
$ go get https://github.com/walter-cd/walter
$ cd $GOPATH/src/github.com/walter-cd/walter
# Add the remote containing customized version of walter
$ git remote add kscherer https://github.com/kscherer/walter
# Fetch changes
$ git fetch kscherer
$ git checkout stages
# Build Walter, this creates the walter binary under bin/
$ make build
```

## Test

Integration tests in `lib/pipeline/pipeline_integration_test.go` can be run using [gotestsum](https://github.com/gotestyourself/gotestsum) by the makefile command
```bash
$ make runtest
```
There are three configurable variables available for the `runtest` task
- **VERBOSE**: Default to `0`. When set to `0`, the format to run `gotestsum` is `short-verbose`. When set to other values,
the format to run `gotestsum` is `standard-verbose`, which is equivalent to `go test -v`
- **NO_CACHE**: Default to `0`. When set to `0`, test will not show cached result and will be run again. When set to other values,
test will show cached result and quit.
- **TEST_CASE**: Default to empty string. Specifies the test case in `pipeline_integration_test.go` to be run. If not set all tests will be run.

The variables can be set by the form `NAME=1`, e.g.
```bash
make runtest VERBOSE=1 TEST_CASE=TestSerial
```

### Test cases

#### Pass tests
Entry point: `test/pipeline.yml`. The following stages are available in the file:
- **Serial**: A set of tasks running in serial order.
- **Directory**: Specifies the directory to run a task.
- **Parallel**: (`parallel.yml`): A set of tasks running in serial order with the `Parallel` task containing a set of subtasks running in parallel.
- **Parallel Serial**: (`parallel_serial.yml`): Similar to **Parallel** with the `Serial` task in `Parallel` task containing a set of subtasks running in serial order.
- **Parallel Serial Parallel**: (`parallel_serial_parallel.yml`): Similar to **Parallel Serial** with the `Serial Parallel` task in `Serial` task in `Parallel` task containing a set of subtasks running in parallel.
- **Script**: Runs the `good.sh` script.
- **Wait For**: Echos out "3 seconds passed" after three seconds passed.

#### Fail tests
Entry point: `test/pipeline_fail.yml`. The following stages are available in the file and is expected to fail due to running a non-existent command. All subsequent steps should be skipped after a failure and `Cleanup` stage should run:
- **Serial Fail**: One tasks fails in a set of tasks running in serial order.
- **Directory Fail**: Specifies an invalid directory to run a task.
- **Parallel Fail**: (`parallel_fail.yml`): A set of tasks running in serial order with the `Parallel` task containing a set of subtasks running in parallel, one of the parallel subtask fails.
- **Parallel Serial Fail**: (`parallel_serial_fail.yml`): Similar to **Parallel** with the `Serial` task in `Parallel` task containing a set of subtasks running in serial order, one of the serial subtask in the parallel task fails.
- **Parallel Serial Parallel Fail**: (`parallel_serial_parallel_fail.yml`): Similar to **Parallel Serial** with the `Serial Parallel` task in `Serial` task in `Parallel` task containing a set of subtasks running in parallel, one of the serial subtask in the parallel subtask fails.
- **Clean Fail**: A stage with invalid cleanup command

#### Interrupt tests
Entry point: `test/pipeline_interrupt.yml`. The following stages are available in the file and will be interrupted after 500ms after startup. `Cleanup` stage should run after interrupt and remove any `.tmp` generated.
- **Serial Interrupt**: Interrupts a set of tasks running in serial order.
- **Parallel Interrupt**: (`parallel_interrupt.yml`): A set of tasks running in serial order with the `Parallel` task containing a set of subtasks running in parallel. Interrupt happens during Parallel task execution.
- **Parallel Serial Interrupt**: (`parallel_serial_interrupt.yml`): Similar to **Parallel** with the `Serial` task in `Parallel` task containing a set of subtasks running in serial order. Interrupt happens during the serial subtask execution.
- **Parallel Serial Parallel Interrupt**: (`parallel_serial_parallel_interrupt.yml`): Similar to **Parallel Serial** with the `Serial Parallel` task in `Serial` task in `Parallel` task containing a set of subtasks running in parallel. Interrupt happens during the serial subtask execution in the parallel tasks.
- **Run Good Script**: Runs the `good.sh` script. After interrupt, "GOODTEST" is echoed and exits normally.
- **Run Bad Script**: Runs the `bad.sh` script. After interrupt, "BADTEST" is echoed and hangs (sleeps) for 60 seconds. Walter should kill the program after a certain timeout (default is 5 seconds) in test `TestInterruptTimeout` or kill the program after a second interrupt in test `TestInterruptForce`.
- **Good And Bad**: Runs both `good.sh` and `bad.sh`.
- **Abort Task**: Creates `TaskAbort1.tmp` and runs `abort.sh`. Interrupt happens during the execution of `abort.sh`. After interrupt, `TaskAbort1.tmp` is deleted. This stage does not have cleanup.
- **Parallel Abort**: Run two `Abort Task` in parallel. After interrupt `TaskAbort1.tmp` and `TaskAbort2.tmp` are deleted. This stage does not have cleanup.
