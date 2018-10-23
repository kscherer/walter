package main

import (
	"flag"
	"os"

	log "github.com/Sirupsen/logrus"

	"github.com/walter-cd/walter/lib/pipeline"
)

func main() {
	const defaultConfigFile = "pipeline.yml"

	var (
		configFile string
		version    bool
		stage      string
	)

	flag.StringVar(&configFile, "config", defaultConfigFile, "file which define pipeline")
	flag.BoolVar(&version, "version", false, "print version string")
	flag.StringVar(&stage, "stage", "", "select the stage to run")

	flag.Parse()

	if version {
		log.Info(OutputVersion())
		os.Exit(0)
	}

	p, err := pipeline.LoadFromFile(configFile)
	if err != nil {
		log.Fatal(err)
	}

	os.Exit(p.Run(stage))
}
