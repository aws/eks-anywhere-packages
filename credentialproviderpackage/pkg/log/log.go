package log

import (
	"log"
	"os"
)

var (
	InfoLogger    *log.Logger
	WarningLogger *log.Logger
	ErrorLogger   *log.Logger
)

func init() {
	InfoLogger = log.New(os.Stdout, "INFO: ", log.Lshortfile)
	WarningLogger = log.New(os.Stderr, "WARNING: ", log.Lshortfile)
	ErrorLogger = log.New(os.Stderr, "ERROR: ", log.Lshortfile)
}
