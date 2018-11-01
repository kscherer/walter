package main

import (
	"flag"
	"os"

	log "github.com/Sirupsen/logrus"

	"github.com/gofrs/uuid"
	"github.com/walter-cd/walter/lib/pipeline"
)

func main() {
	const defaultConfigFile = "pipeline.yml"

	var (
		configFile string
		version    bool
		stage      string
		buildID    string
	)

	flag.StringVar(&configFile, "config", defaultConfigFile, "file which define pipeline")
	flag.BoolVar(&version, "version", false, "print version string")
	flag.StringVar(&stage, "stage", "", "select the stage to run")
	flag.StringVar(&buildID, "build_id", "", "specify the build id to use. Default random uuid")

	flag.Parse()

	if version {
		log.Info(OutputVersion())
		os.Exit(0)
	}

	p, err := pipeline.LoadFromFile(configFile)
	if err != nil {
		log.Fatal(err)
	}

	if buildID == "" {
		uuid, err := uuid.NewV4()
		if err != nil {
			log.Fatal("failed to generate UUID: %v", err)
		}
		buildID = uuid.String()
	}

	os.Exit(p.Run(stage, buildID))
}
