package audio

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

var logFile *os.File

// initLogging opens a timestamped log file in the user’s home directory (~/.gowav/logs).
func initLogging() error {
	if logFile != nil {
		return nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	logDir := filepath.Join(homeDir, ".gowav", "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return err
	}

	logPath := filepath.Join(logDir, fmt.Sprintf("gowav_%s.log",
		time.Now().Format("2006-01-02_15-04-05")))

	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	logFile = f
	return nil
}

// logDebug writes a debug-level message to the log file, if it’s successfully opened.
func logDebug(format string, args ...interface{}) {
	if err := initLogging(); err != nil {
		return
	}
	timestamp := time.Now().Format("15:04:05.000")
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(logFile, "[%s] %s\n", timestamp, msg)
}
