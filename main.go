package main

import (
	lib "cdddru-tool/packages/cdddru"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
)

func main() {
	var logger *lib.Logger
	var wg sync.WaitGroup
	startupLogger := lib.NewLogger(os.Stdout, os.Stderr, lib.DebugLevel, "Startup")

	lib.Mode = lib.GetEnvVar("MODE", "production")

	// run initialization, detect configs and input parameters
	jobs, err := lib.Startup(startupLogger)
	if err != nil {
		lib.CheckIfError(startupLogger, err, true)
	}
	if len(jobs) == 0 {
		lib.CheckIfError(startupLogger, fmt.Errorf("error reading configs: %v", "no configs have been read"), true)
	}

	fmt.Println("Job's quantity:", len(jobs))
	for _, job := range jobs {
		fmt.Println("Job's Name:", job.COMMON.JOB_NAME)
	}

	// firstJob := jobs[0]
	// seconfJob := lib.Config{}
	// if len(jobs) > 1 {
	// 	seconfJob = jobs[1]
	// }
	lib.InlineTest(false, *jobs[0], logger, true)

	for _, job := range jobs {
		wg.Add(1)
		go lib.RunOneJob(job, &wg)
	}

	sigCh := make(chan os.Signal, 1)
	// Notify the sigCh channel when a SIGINT signal is received (Ctrl-C)
	signal.Notify(sigCh, syscall.SIGINT)

	go func() {
		<-sigCh
		CtrCHandler()
	}()

	// wg.Add(1)
	// go lib.RunOneJob(firstJob, &wg)
	// wg.Add(1)
	// go lib.RunOneJob(seconfJob, &wg)
	wg.Wait()
}

func CtrCHandler() {
	err := os.Remove(filepath.Join(os.Getenv("HOME"), ".docker", "config.json"))
	if err != nil {
		fmt.Println("docker config delete error", err)
	}
	os.Exit(130)
}
