package cdddru

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"golang.org/x/exp/slices"
	"gopkg.in/yaml.v3"
)

var FbOnce bool
var FbVerbose bVerbose

var Mode string

func Startup(logger *Logger) ([]Config, error) {

	// Define named flags
	fsJobFolder := flag.String("jobsfolder", "", "Input directory with jobs to execute")
	flag.StringVar(fsJobFolder, "j", "", "Input directory with jobs to execute (Jobfolder)")

	fsJobFile := flag.String("jobfile", "", "Input file with specified job")
	flag.StringVar(fsJobFile, "f", "", "Input file with specified job (jobfile)")

	fsJobName := flag.String("jobname", "", "Job name in specified folder or job file")
	flag.StringVar(fsJobName, "n", "", "Job name in specified folder or job file (jobname)")

	fnDelaySec := flag.Int("delay", 30, "Time to pause befor next job will starts")
	flag.IntVar(fnDelaySec, "d", 30, "Time to pause befor next job will starts (delay)")

	flag.BoolVar((*bool)(&FbVerbose), "verbose", false, "verbose output")
	flag.BoolVar((*bool)(&FbVerbose), "v", false, "verbose output")

	flag.BoolVar(&FbOnce, "oncerun", false, "once running and exit")

	jobsConfigs := make([]Config, 0)
	err := ParseFlags()
	if err != nil {
		return jobsConfigs, fmt.Errorf("failed to parse cmdline args and named parameters: %v", err)
	}

	// if len(os.Args) == 1 {
	// 	flag.Usage()
	// 	os.Exit(1)
	// }

	// Parse positional arguments
	args := flag.Args()

	// fmt.Println("args:", args)

	// if len(args) > 0 {

	// }
	// if len(args) > 1 {
	// 	*fsJobFile = args[1]
	// }
	// if len(args) > 2 {configs[1].JOB_NAME
	// 	*fsJobName = args[2]
	// }

	if len(args) == 0 && len(*fsJobFile) == 0 && len(*fsJobFolder) == 0 {
		usr, err := user.Current()
		if err != nil {
			return jobsConfigs, fmt.Errorf("failed to get current user: %v", err)
		}
		executable, err := os.Executable()

		if err != nil {
			return jobsConfigs, fmt.Errorf("failed to get executable: %v", err)
		}

		if strings.Contains(executable, "go-build") {
			// Running as a script using 'go run'
			*fsJobFile = filepath.Join(usr.HomeDir, ".config", "cdddru", "config.json")
			os.MkdirAll(filepath.Dir(*fsJobFile), os.ModePerm)
			if err != nil {
				return jobsConfigs, fmt.Errorf("failed to create config path for current user: %v", err)
			}
		} else {
			// Running as a binary
			*fsJobFile = filepath.Join(filepath.Dir(executable), "config.json")
		}
	}

	// fmt.Println("fsJobFolder:", *fsJobFolder, "\tfsJobFile:", *fsJobFile, "\tfsJobName:", *fsJobName)

	if len(*fsJobFolder) > 0 {
		// walk through specified Dir and make array of Configs
		// len(*fsJobFile) == 0 &&
		err := filepath.Walk(*fsJobFolder, func(wPath string, info os.FileInfo, err error) error {
			// if the same path
			var err2 error
			if wPath == *fsJobFolder {
				return nil
			}
			// If current path is Dir - do nothing
			if info.IsDir() {
				_ = fmt.Sprintf("[%s]\n", wPath)
			}
			// if we got file, we take its full path and
			if wPath != *fsJobFolder && !info.IsDir() {
				fullConfigFilePath := wPath
				match := true

				jobFilePattern := *fsJobFile
				filePath := filepath.Base(fullConfigFilePath)
				if len(jobFilePattern) > 0 {
					match, err2 = filepath.Match(jobFilePattern, filePath)
					if err2 != nil {
						fmt.Println(err2)
					}
					if FbVerbose {
						PrintDebug(logger, "match?: %v, jobFilePattern: %v, filePath: %v", match, jobFilePattern, filePath)
					}
				}
				if match {
					config, err2 := getOneConfig(fullConfigFilePath)
					if err2 != nil {
						return err
					}
					idx := slices.IndexFunc(jobsConfigs, func(c Config) bool { return c.JOB_NAME == config.JOB_NAME })
					if idx >= 0 {
						err2 = fmt.Errorf("job '%s' already presented in slice", config.JOB_NAME)
						return err2
					}
					jobsConfigs = append(jobsConfigs, *config)

				}
			}
			return err2
		})

		if err != nil {
			return make([]Config, 0), err
		}
		return jobsConfigs, nil
	}

	// if path to config file specified and no folderpath specified
	if (len(*fsJobFile) > 0 && len(*fsJobFolder) == 0) || len(args) > 0 {

		var configPath string
		var config *Config
		var err error
		if len(*fsJobFile) > 0 && len(*fsJobFolder) == 0 {
			// calculating configPath
			configPath = filepath.Join(*fsJobFile)
			if len(*fsJobFolder) > 0 {
				configPath = filepath.Join(*fsJobFolder, *fsJobFile)
			}
			config, err = getOneConfig(configPath)
			if err != nil {
				return jobsConfigs, err
			}
			jobsConfigs = append(jobsConfigs, *config)
		}
		if len(args) > 0 {
			for _, cfgPath := range args {
				config, err := getOneConfig(cfgPath)
				fmt.Println("config from args:", config, err)
				if err != nil {
					return jobsConfigs, err
				}
				jobsConfigs = append(jobsConfigs, *config)
			}
		}
	}
	return jobsConfigs, nil
}

func getOneConfig(configPath string) (*Config, error) {
	// Read the config file
	configFileBytes, err := os.ReadFile(configPath)

	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return &Config{}, fmt.Errorf("failed to read config file %s: %v", configPath, err)
	} else if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}

	configFile, _ := ReplaceEnvs(string(configFileBytes))

	var config Config
	// Parse the config file
	fileName := filepath.Base(configPath)
	fileExt := strings.ToLower(filepath.Ext(fileName))
	// fileBaseName := fileName[:len(fileName)-len(fileExt)]

	switch fileExt {
	case ".yaml":
		err = yaml.Unmarshal([]byte(configFile), &config)

	case ".json":
		err = json.Unmarshal([]byte(configFile), &config)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to parse config file: %v", err)
	}
	if Mode != "development" && config.CHECK_INTERVAL < 120 {
		config.CHECK_INTERVAL = 120
	}
	return &config, nil
}
