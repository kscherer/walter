Walter
======

## Build

```bash
# Get walter from source
$ go get https://github.com/walter-cd/walter
$ cd $GOPATH/src/github.com/walter-cd/walter
# Add the remote containing customized version of walter
$ git remote add kschere https://github.com/kschere/walter
# Fetch changes
$ git fetch kschere
$ git checkout stages
# Build Walter, this creates the walter binary under bin/
$ make build
```

## Test

### In Commandline

The entry point of tests is the `pipeline.yml` file inside `test/` directory. The following stages are available in the file:
- **Serial**: A set of tasks running in serial order.
- **Parallel** (`parallel.yml`): A set of tasks running in serial order with the `Parallel` task containing a set of subtasks running in parallel.
- **Parallel Serial** (`parallel_serial.yml`): Similar to **Parallel** with the `Serial` task in `Parallel` task containing a set of subtasks running in serial order.
- **Parallel Serial Parallel** (`parallel_serial_parallel.yml`): Similar to **Parallel Serial** with the `Serial Parallel` task in `Serial` task in `Parallel` task containing a set of subtasks running in parallel.
- **Run Good Script**: Runs the `good.sh` script which, if interrupted, echos out "GOODTEST" and exits.
- **Run Bad Script**: Runs the `bad.sh` script which, if interrupted, echos out "BADTEST" and hangs (sleeps) for 60 seconds. Walter should kill the program after a certain timeout (default is 5 seconds).
- **Good And Bad**: Runs both `good.sh` and `bad.sh`
- **Wait For**: Echos out "3 seconds passed" after three seconds passed.

Test could be run by
```bash
$ ./bin/walter --config test/pipeline.yml
```
The above command will run all stages in `pipeline.yml`.

Running a specific stage is also possible:
```bash
$ ./bin/walter --config test/pipeline.yml --stage "$STAGE_NAME"
```

### Using Scripts

The `test.sh` and `testInterrupt.sh` scripts could also be used. Note that they must be run in the project root directory.

`test.sh` runs `walter` without any interrupt, it could take stage name as an optional parameter. If the parameter is not set, all stages will be run.
```bash
$ ./test/test.sh [STAGE_NAME]
```

`testInterrupt.sh` runs `walter` and interrupts it after a certain amount of time. In addition to the stage name, `testInterrupt.sh` takes a second parameter as the time in seconds before it interrupts `walter`.
```bash
$ ./test/testInterrupt.sh [STAGE_NAME] [TIME_BEFORE_INTERRUPT]
```

### Race Condition Detection

Test could be run with race condition detection as well:
```bash
$ go run -race main.go version.go --config test/pipeline.yml --stage ${STAGE}
```
Note that this command does not recompile the binary but uses the `.go` files directly.

