package cdddru

import (
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/fatih/color"
)

type LogLevel int

const (
	DebugLevel LogLevel = iota
	InfoLevel
	WarnLevel
	ErrorLevel
)

type Logger struct {
	debugLogger *log.Logger
	infoLogger  *log.Logger
	warnLogger  *log.Logger
	errorLogger *log.Logger
	fatalLogger *log.Logger
	logLevel    LogLevel
}

func NewLogger(out io.Writer, outError io.Writer, level LogLevel, jobName string) *Logger {
	if outError == nil {
		outError = out
	}
	debugColor := color.New(color.FgHiCyan).SprintFunc()
	infoColor := color.New(color.FgWhite).SprintFunc()
	warnColor := color.New(color.FgHiYellow).SprintFunc()
	errorColor := color.New(color.FgHiRed).SprintFunc()
	fatalColor := color.New(color.FgHiMagenta).SprintFunc()

	debugLogger := log.New(out, debugColor(jobName+"->| DEBUG: "), log.Flags()|log.Llongfile)
	warnLogger := log.New(out, warnColor(jobName+"->| WARNING: "), log.LstdFlags)
	infoLogger := log.New(out, infoColor(jobName+"->| INFO: "), log.LstdFlags)
	errorLogger := log.New(outError, errorColor(jobName+"->| ERROR: "), log.LstdFlags)
	fatalLogger := log.New(outError, fatalColor(jobName+"->| FATAL: "), log.LstdFlags)

	return &Logger{
		debugLogger: debugLogger,
		infoLogger:  infoLogger,
		warnLogger:  warnLogger,
		errorLogger: errorLogger,
		fatalLogger: fatalLogger,
		logLevel:    level,
	}
}

func (l *Logger) Debug(msg string) {
	if len(strings.TrimSpace(msg)) == 0 {
		msg = "[passed empty message to logger]"
	}
	if l.logLevel <= DebugLevel {
		l.debugLogger.Println(msg)
	}
}

func (l *Logger) Warning(msg string) {
	if msg == "" {
		msg = "[passed empty message to logger]"
	}

	if l.logLevel <= WarnLevel {
		l.warnLogger.Println(msg)
	}
}

func (l *Logger) Info(msg string) {
	if msg == "" {
		msg = "[passed empty message to logger]"
	}

	if l.logLevel <= InfoLevel {
		l.infoLogger.Println(msg)
	}
}

func (l *Logger) InfoJson(msg string) {
	if msg == "" {
		msg = "[passed empty message to logger]"
	}

	jsonMsg, err := PrettyJsonEncodeToString(msg)
	if err != nil {
		l.Error(err.Error())
	} else {
		l.Info(jsonMsg)
	}

}

func (l *Logger) DebugJson(msg string) {
	if msg == "" {
		msg = "[passed empty message to logger]"
	}

	jsonMsg, _ := PrettyJsonEncodeToString(msg)
	l.Debug(jsonMsg)
}

func (l *Logger) WarningJson(msg string) {
	if msg == "" {
		msg = "[passed empty message to logger]"
	}

	jsonMsg, _ := PrettyJsonEncodeToString(msg)
	l.Warning(jsonMsg)
}

func (l *Logger) ErrorJson(msg string) {
	if msg == "" {
		msg = "[passed empty message to logger]"
	}

	jsonMsg, err := PrettyJsonEncodeToString(msg)
	if err != nil {
		l.Error(err.Error())
	} else {
		l.Error(jsonMsg)
	}
}

func (l *Logger) Error(msg string) {
	if msg == "" {
		msg = "[passed empty message to logger]"
	}

	if l.logLevel <= ErrorLevel {
		l.errorLogger.Println(msg)
	}
}

func (l *Logger) Fatal(msg string) {
	if msg == "" {
		msg = "[passed empty message to logger]"
	}

	if l.logLevel <= ErrorLevel {
		l.fatalLogger.Println(msg)
	}
}

// CheckIfError should be used to naively panics if an error is not nil.
func CheckIfError(logger *Logger, err error, isExit bool) {
	if err == nil {
		return
	}
	if isExit {
		logger.Fatal(err.Error())
		panic(err)
	} else {
		logger.Error(err.Error())
	}
}
func CheckIfErrorFmt(logger *Logger, err, errfmt error, isExit bool) error {
	if err == nil {
		return nil
	}
	if errfmt == nil {
		errfmt = err
	}
	if isExit {
		logger.Fatal(errfmt.Error())
		// panic(errfmt)
		return errfmt
	} else {
		logger.Error(errfmt.Error())
		return nil
	}

}

func PrintDebug(logger *Logger, format string, args ...interface{}) {
	logger.Debug(fmt.Sprintf(format, args...))
}

func PrintInfo(logger *Logger, format string, args ...interface{}) {
	logger.Info(fmt.Sprintf(format, args...))
}

// PrintWarning should be used to display a warning
func PrintWarning(logger *Logger, format string, args ...interface{}) {
	logger.Warning(fmt.Sprintf(format, args...))
	// fmt.Printf("\x1b[36;1m%s\x1b[0m\n", fmt.Sprintf(format, args...))
}

// PrintWarning should be used to display a warning
func PrintError(logger *Logger, format string, args ...interface{}) {
	logger.Error(fmt.Sprintf(format, args...))
	// fmt.Printf("\x1b[31;1m%s\x1b[0m\n", fmt.Sprintf(format, args...))
}

func PrintFatal(logger *Logger, format string, args ...interface{}) {
	logger.Fatal(fmt.Sprintf(format, args...))
	// fmt.Printf("\x1b[31;1m%s\x1b[0m\n", fmt.Sprintf(format, args...))
}
