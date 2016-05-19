package seaeye

import (
	"fmt"
	"io"
	"log"
	"os"
	"path"
)

// FileLogger is a log holding a reference to a file meant to log to.
type FileLogger struct {
	*log.Logger
	outFile *os.File
}

// NewFileLogger instantiates a new file logger which additionally to the
// standard logger behaviour also logs to a file. The caller is responsible for
// closing the file handle.
func NewFileLogger(filePath, prefix string, flag int) (*FileLogger, error) {
	logFile, err := createFile(filePath)
	if err != nil {
		return nil, err
	}

	w := io.MultiWriter(os.Stderr, logFile)

	logger := &FileLogger{
		Logger:  log.New(w, prefix, flag),
		outFile: logFile,
	}

	return logger, nil
}

func createFile(logFilePath string) (*os.File, error) {
	if err := os.MkdirAll(path.Dir(logFilePath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create directories: %v", err)
	}

	logFile, err := os.Create(logFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create file: %v", err)
	}

	return logFile, nil
}
