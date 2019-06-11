package main

import (
	"github.com/CodingBeard/go-logger"
	"os"
)

func main () {
	// Get the instance for logger class
	// Third option is optional and is instance of type io.Writer, defaults to os.Stderr
	log, err := logger.New("test", 1, os.Stdout)
	if err != nil {
		panic(err) // Check for error
	}

	// Critically log critical
	log.Critical("category","This is Critical!")
	// Debug
	log.Debug("category","This is Debug!")
	// Give the Warning
	log.Warning("category","This is Warning!")
	// Show the error
	log.Error("category","This is Error!")
	// Notice
	log.Notice("category","This is Notice!")
	// Show the info
	log.Info("category","This is Info!")

	// Show warning with format message
	log.SetFormat("[%{module}] [%{level}] %{message}")
	log.Warning("category","This is Warning!")
	// Also you can set your format as default format for all new loggers
	logger.SetDefaultFormat("%{message}")
	log2, _ := logger.New("pkg", 1, os.Stdout)
	log2.Error("category","This is Error!")
}