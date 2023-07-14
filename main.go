package main

import (
	lib "cdddru-tool/packages/cdddru"
	"fmt"
	"os"
	"sync"
)

func main() {
	var logger *lib.Logger
	var wg sync.WaitGroup
	startupLogger := lib.NewLogger(os.Stdout, os.Stderr, lib.DebugLevel, "Startup")

	// run initialization, detect configs and input parameters
	configs, err := lib.Startup(startupLogger)
	if err != nil {
		lib.CheckIfError(startupLogger, err, true)
	}
	if len(configs) == 0 {
		lib.CheckIfError(startupLogger, fmt.Errorf("error reading configs: %v", "no configs have been read"), true)
	}

	config := configs[0]
	lib.InlineTest(false, config, logger, true)
	wg.Add(1)
	go lib.RunOneJob(config, &wg)
	wg.Wait()
}
