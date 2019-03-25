package logger

import (
	"fmt"
	"os"
	"strings"
	"time"
	"sync"
	"io"
	"bytes"
)

var (
	logfile 							*os.File
	lock     							sync.Mutex
	filename 							string
	mOperationsLogFileLinesNumber		int
	mMaxOperationsLogFileLinesNumber 	int
	mOnRotateStage						int
)

func write(group, message string) {

	message = strings.TrimRight(message, "\n")
	if len(message) > 0 && message[len(message)-1] != '.' {
		message += "."
	}

	logRecord := fmt.Sprintln(time.Now().UTC().Format(time.RFC3339), group, message)

	if logfile == nil {
		println("File logger: can't write log record because logger isn't initialised yet.")
		println(group, message)

	} else {
		// Writing to file
		symbolsWritten, err := logfile.Write([]byte(logRecord))
		if symbolsWritten == 0 || err != nil {
			println("File logger: can't write log record. Details: " + err.Error())
		}
		mOperationsLogFileLinesNumber++
	}

	if mOperationsLogFileLinesNumber >= mMaxOperationsLogFileLinesNumber && mOnRotateStage == 0 {
		err := rotate()
		mOnRotateStage = 0
		if err != nil {
			println("File logger: can't rotate log file. Details: " + err.Error())
		} else {
			mOperationsLogFileLinesNumber = 0
		}
	}
}

// Perform the actual act of rotating and reopening file.
func rotate() error {
	lock.Lock()
	defer lock.Unlock()
	mOnRotateStage = 1

	// Close existing file if open
	if logfile != nil {
		err := logfile.Close()
		logfile = nil
		if err != nil {
			println("Can't close previous log")
			return err
		}
	}
	// Rename dest file if it already exists
	_, err := os.Stat(filename)
	if err == nil {
		err = os.Rename(filename, "rotate_" + filename+"."+time.Now().Format(time.RFC3339Nano) + ".log")
		if err != nil {
			println("Can't rename old log")
			return err
		}
	}

	// Create a file.
	flag := os.O_APPEND | os.O_WRONLY | os.O_CREATE
	logfile, err = os.OpenFile(filename, flag, 0600)
	return err
}

func Init() error {
	flag := os.O_APPEND | os.O_WRONLY | os.O_CREATE
	filename = "operations.events.log"

	var err error
	logfile, err = os.OpenFile(filename, flag, 0600)
	if err != nil {
		return err
	}

	mOperationsLogFileLinesNumber, err = logLineCounter();
	if err != nil {
		println("Can't calculate log lines count")
		mOperationsLogFileLinesNumber = 0
	}
	mMaxOperationsLogFileLinesNumber = 500000;
	mOnRotateStage = 0

	return nil
}

func logLineCounter() (int, error) {
	flag := os.O_RDONLY
	logfile, err := os.OpenFile(filename, flag, 0600)
	if err != nil {
		return 0, err
	}
	defer logfile.Close()

	buf := make([]byte, 32*1024)
	count := 0
	lineSep := []byte{'\n'}

	for {
		c, err := logfile.Read(buf)
		count += bytes.Count(buf[:c], lineSep)

		switch {
		case err == io.EOF:
			return count, nil

		case err != nil:
			return count, err
		}
	}
}

func Error(message string) {
	write("\tERROR\t", message)
}

func Info(message string) {
	write("\tINFO\t", message)
}

func Debug(message string) {
	write("\tDEBUG\t", message)
}
