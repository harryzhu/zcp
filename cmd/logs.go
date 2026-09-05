package cmd

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var logHandler *os.File
var logWriter *bufio.Writer

func StartLogging(fname string) {
	var err error
	logPath := ToUnixSlash(filepath.Join(LogDir, fname))
	log.Println("logPath: ", logPath)
	logHandler, err = os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, os.ModePerm)
	if err != nil {
		log.Fatal(err)
	}
	logWriter = bufio.NewWriter(logHandler)
}

func StopLogging() {
	time.Sleep(time.Second)
	logWriter.Flush()
	logHandler.Close()
}

func WriteLog(cls string, prefix string, msg string) error {
	ts := time.Now().Format("2006-01-02 15:04:05")
	line := fmt.Sprintf("\n%s %s %s %v", ts, cls, prefix, msg)

	BufWriteFile(logHandler, []byte(line))
	return nil
}

func BufWriteFile(fp io.Writer, data []byte) error {
	r := bytes.NewReader(data)
	_, err := logWriter.ReadFrom(r)
	if err != nil {
		PrintError("BufWriteFile", err)
		return err
	}
	logWriter.Flush()
	return nil
}

func FatalError(prefix string, err error) {
	if err != nil {
		go WriteLog("ERROR", prefix, err.Error())
		log.Fatal(Red(strings.Join([]string{"ERROR", prefix, ""}, ": ")), err)
	}
}

func NewError(args ...any) error {
	var s []string
	for _, arg := range args {
		s = append(s, fmt.Sprintf("%v", arg))
	}
	return errors.New(strings.Join(s, ""))
}

func DebugInfo(prefix string, args ...any) {
	if IsDebug {
		var info []string
		for _, arg := range args {
			info = append(info, fmt.Sprintf("%v", arg))
		}
		line := strings.Join(info, "")
		go WriteLog("DEBUG", prefix, line)
		log.Printf("DEBUG: %v: %v\n", prefix, line)
	}
}

func DebugWarn(prefix string, args ...any) {
	if IsDebug {
		var info []string
		for _, arg := range args {
			info = append(info, fmt.Sprintf("%v", arg))
		}
		line := strings.Join(info, "")
		go WriteLog("WARN", prefix, line)
		log.Println(Yellow("WARN:"), Yellow(prefix+":"), Yellow(line))
	}
}

func PrintError(prefix string, err error) {
	if err != nil {
		go WriteLog("ERROR", prefix, err.Error())
		log.Println(Red("ERROR:"), Red(prefix), err)
	}
}

func PrintlnInfo(color string, prefix string, args ...any) {
	var info []string
	for _, arg := range args {
		info = append(info, fmt.Sprintf("%v", arg))
	}
	line := strings.Join(info, "")
	go WriteLog("INFO", prefix, line)

	switch strings.ToLower(color) {
	case "green":
		log.Printf("INFO: %v: %v\n", Green(prefix), line)
	case "black":
		log.Printf("INFO: %v: %v\n", Black(prefix), line)
	case "red":
		log.Printf("INFO: %v: %v\n", Red(prefix), line)
	case "yellow":
		log.Printf("INFO: %v: %v\n", Yellow(prefix), line)
	case "blue":
		log.Printf("INFO: %v: %v\n", Blue(prefix), line)
	case "purple":
		log.Printf("INFO: %v: %v\n", Purple(prefix), line)
	case "cyan":
		log.Printf("INFO: %v: %v\n", Cyan(prefix), line)
	case "white":
		log.Printf("INFO: %v: %v\n", White(prefix), line)
	default:
		log.Printf("INFO: %v: %v\n", prefix, line)
	}

}

// -----color----
const (
	textBlack = iota + 30
	textRed
	textGreen
	textYellow
	textBlue
	textPurple
	textCyan
	textWhite
)

func Black(str string) string {
	return textColor(textBlack, str)
}

func Red(str string) string {
	return textColor(textRed, str)
}
func Yellow(str string) string {
	return textColor(textYellow, str)
}
func Green(str string) string {
	return textColor(textGreen, str)
}
func Cyan(str string) string {
	return textColor(textCyan, str)
}
func Blue(str string) string {
	return textColor(textBlue, str)
}
func Purple(str string) string {
	return textColor(textPurple, str)
}
func White(str string) string {
	return textColor(textWhite, str)
}

func textColor(color int, str string) string {
	return fmt.Sprintf("\x1b[0;%dm%s\x1b[0m", color, str)
}

func PrintSpinner(s string) {
	fmt.Printf(" ... %20.10s\r", s)
}

func PrintSpinner2(s string, suffix string) {
	fmt.Printf(" ... %5.10s    %s\r", s, suffix)
}
