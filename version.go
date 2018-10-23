package main

import (
	"bytes"
	"fmt"
	"time"
)

// Name is application name
const Name = "walter"

// Version is application version
const Version string = "v2.0.0"

// GitCommit describes latest commit hash.
// This is automatically extracted by git describe --always.
var GitCommit string

const defaultCheckTimeout = 2 * time.Second

// OutputVersion display version number
func OutputVersion() string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "%s version %s", Name, Version)
	if len(GitCommit) != 0 {
		fmt.Fprintf(&buf, " (%s)", GitCommit)
	}
	fmt.Fprintf(&buf, "\n")
	return buf.String()
}
