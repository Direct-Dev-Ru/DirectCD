package main

import (
	"fmt"
	"io"
	"log"
	"os"

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
	logLevel    LogLevel
}

func NewLogger(out io.Writer, outError io.Writer, level LogLevel, taskName string) *Logger {
	debugColor := color.New(color.FgHiCyan).SprintFunc()
	infoColor := color.New(color.FgWhite).SprintFunc()
	warnColor := color.New(color.FgHiYellow).SprintFunc()
	errorColor := color.New(color.FgHiRed).SprintFunc()

	debugLogger := log.New(out, debugColor("TASK --> "+taskName+" -| LEVEL-DEBUG: "), log.Flags()|log.Llongfile)
	warnLogger := log.New(out, warnColor("TASK --> "+taskName+" -| LEVEL-WARNING: "), log.LstdFlags)
	infoLogger := log.New(out, infoColor("TASK --> "+taskName+" -| LEVEL-INFO: "), log.LstdFlags)
	if outError == nil {
		outError = out
	}
	errorLogger := log.New(outError, errorColor("TASK: "+taskName+" -| LEVEL-ERROR: "), log.LstdFlags)

	return &Logger{
		debugLogger: debugLogger,
		infoLogger:  infoLogger,
		warnLogger:  warnLogger,
		errorLogger: errorLogger,
		logLevel:    level,
	}
}

func (l *Logger) Debug(msg string) {
	if l.logLevel <= DebugLevel {
		l.debugLogger.Println(msg)
	}
}

func (l *Logger) Warning(msg string) {
	if l.logLevel <= WarnLevel {
		l.warnLogger.Println(msg)
	}
}

func (l *Logger) Info(msg string) {
	if l.logLevel <= InfoLevel {
		l.infoLogger.Println(msg)
	}
}

func (l *Logger) Error(msg string) {
	if l.logLevel <= ErrorLevel {
		l.errorLogger.Printf("\x1b[31;1m%s\x1b[0m\n", msg)
	}
}

// CheckIfError should be used to naively panics if an error is not nil.
func CheckIfError(logger *Logger, err error, isExit bool) {
	if err == nil {
		return
	}

	logger.Error(err.Error())
	if isExit {
		os.Exit(1)
	}
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
