package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
)

var fbOnce bool
var fbVerbose bVerbose

func startup() ([]Config, error) {

	// Define named flags
	fsTaskFolder := flag.String("taskfolder", "", "Input directory with tasks to execute")
	flag.StringVar(fsTaskFolder, "t", "", "Input directory with tasks to execute (taskfolder)")

	fsTaskFile := flag.String("taskfile", "", "Input file with specified task")
	flag.StringVar(fsTaskFile, "f", "", "Input file with specified task (taskfile)")

	fsTaskName := flag.String("taskname", "", "Task name in specified folder or task file")
	flag.StringVar(fsTaskName, "n", "", "Task name in specified folder or task file (taskname)")

	fnDelaySec := flag.Int("delay", 30, "Time to pause befor next task will starts")
	flag.IntVar(fnDelaySec, "d", 30, "Time to pause befor next task will starts (delay)")

	flag.BoolVar((*bool)(&fbVerbose), "verbose", false, "verbose output")
	flag.BoolVar((*bool)(&fbVerbose), "v", false, "verbose output")

	flag.BoolVar(&fbOnce, "oncerun", false, "once running and exit")

	tasksConfigs := make([]Config, 0)
	err := ParseFlags()
	if err != nil {
		return tasksConfigs, fmt.Errorf("failed to parse cmdline args and named parameters: %v", err)
	}

	// if len(os.Args) == 1 {
	// 	flag.Usage()
	// 	os.Exit(1)
	// }

	// Parse positional arguments
	args := flag.Args()
	if len(args) > 0 {
		*fsTaskFolder = args[0]
	}
	if len(args) > 1 {
		*fsTaskFile = args[1]
	}
	if len(args) > 2 {
		*fsTaskName = args[2]
	}
	if len(args) == 0 && len(*fsTaskFile) == 0 {
		usr, err := user.Current()
		if err != nil {
			return tasksConfigs, fmt.Errorf("failed to get current user: %v", err)
		}
		executable, err := os.Executable()
		fmt.Println(executable)
		if err != nil {
			return tasksConfigs, fmt.Errorf("failed to get executable: %v", err)
		}

		if strings.Contains(executable, "go-build") {
			// Running as a script using 'go run'
			*fsTaskFile = filepath.Join(usr.HomeDir, ".config", "ddru-cd-tool", "config.json")
			os.MkdirAll(filepath.Dir(*fsTaskFile), os.ModePerm)
			if err != nil {
				return tasksConfigs, fmt.Errorf("failed to create config path for current user: %v", err)
			}
		} else {
			// Running as a binary
			*fsTaskFile = filepath.Join(filepath.Dir(executable), "config.json")
		}
	}

	fmt.Println("fsTaskFolder:", *fsTaskFolder, "\tfsTaskFile:", *fsTaskFile, "\tfsTaskName:", *fsTaskName)

	if len(*fsTaskFile) == 0 && len(*fsTaskFolder) > 0 {
		// walk through specified Dir and make array of Configs
		return tasksConfigs, nil
	}

	// if path to config file specified
	if len(*fsTaskFile) > 0 {

		var configPath string
		var config Config = Config{}

		// calculating configPath
		configPath = filepath.Join(*fsTaskFile)
		if len(*fsTaskFolder) > 0 {
			configPath = filepath.Join(*fsTaskFolder, *fsTaskFile)
		}
		config, err := getOneConfig(configPath)
		if err != nil {
			return tasksConfigs, err
		}
		tasksConfigs = append(tasksConfigs, config)
	}
	return tasksConfigs, nil
}

func getOneConfig(configPath string) (Config, error) {
	// Read the config file
	configFileBytes, err := os.ReadFile(configPath)

	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return Config{}, fmt.Errorf("failed to read config file %s: %v", configPath, err)
	} else if errors.Is(err, os.ErrNotExist) {

		return DefaultConfig, nil
	}

	// configFile := strings.TrimSpace(string(configFileBytes))
	// pattern := `{{\$(.*?)}}`

	// // Compile the regular expression
	// regex := regexp.MustCompile(pattern)
	// // Find all matches in the text
	// matches := regex.FindAllStringSubmatch(configFile, -1)

	// // Print the matches
	// for _, match := range matches {
	// 	replacedText := match[0]
	// 	replacingText := getEnvVar(match[1], "")
	// 	configFile = strings.ReplaceAll(configFile, replacedText, replacingText)
	// }

	configFile, _ := replaceEnvs(string(configFileBytes))

	var config Config
	// Parse the config file
	err = json.Unmarshal([]byte(configFile), &config)
	if err != nil {
		return Config{}, fmt.Errorf("failed to parse config file: %v", err)
	}
	if config.CHECK_INTERVAL < 120 {
		config.CHECK_INTERVAL = 120
	}
	return config, nil
}
