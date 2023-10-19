package cdddru

import (
	"fmt"
	"os"
)

func InlineTest(isRun bool, config Config, logger *Logger, isExit bool) {
	if isRun {

		logger.Debug(fmt.Sprint(PrettyJsonEncodeToString(config)))
		// ready, err := getDeploymentReadinessStatus(config, "kuznetcovay/ddru:v1.0.11")
		// if err != nil {
		// 	PrintError(logger, "%v", err)
		// 	os.Exit(1)
		// }
		// logger.Debug(fmt.Sprintf("%v", ready))

		waitApplyingTimeSeconds := 3 * config.COMMON.CHECK_INTERVAL / 4
		intervalToWaitSeconds := waitApplyingTimeSeconds / 5
		checkIntervals := [5]int{intervalToWaitSeconds, intervalToWaitSeconds, intervalToWaitSeconds, intervalToWaitSeconds, intervalToWaitSeconds}
		if intervalToWaitSeconds > 120 {
			checkIntervals[0] = 120
			totalWait := checkIntervals[0]
			for i := 1; i < 4; i++ {
				restWait := waitApplyingTimeSeconds - totalWait
				newInterval := restWait / (5 - i)
				fmt.Println(i, restWait, (5 - i), newInterval, totalWait)
				if newInterval > checkIntervals[i-1]*2 {
					checkIntervals[i] = checkIntervals[i-1] * 2
				} else {
					checkIntervals[i] = newInterval
				}
				totalWait += checkIntervals[i]
			}
			checkIntervals[4] = waitApplyingTimeSeconds - totalWait
		}

		fmt.Println(checkIntervals)

		if isExit {
			os.Exit(0)
		}
	}
}
