// Package name declaration
package logger

// Import packages
import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var (
	// Map for the various codes of colors
	colors map[LogLevel]string

	// Map from format's placeholders to printf verbs
	phfs map[string]string

	// Contains color strings for stdout
	logNo uint64

	// Default format of log message
	defFmt = "#%[1]d %[2]s %[4]s:%[5]d ▶ %.3[6]s %[7]s"

	// Default format of time
	defTimeFmt = "2006-01-02 15:04:05"
)

// LogLevel type
type LogLevel int

// Color numbers for stdout
const (
	Black = (iota + 30)
	Red
	Green
	Yellow
	Blue
	Magenta
	Cyan
	White
)

// Log Level
const (
	CriticalLevel LogLevel = iota + 1
	ErrorLevel
	WarningLevel
	NoticeLevel
	InfoLevel
	DebugLevel
)

// Worker class, Worker is a log object used to log messages and Color specifies
// if colored output is to be produced
type Worker struct {
	Minion     *log.Logger
	Color      int
	format     string
	timeFormat string
	level      LogLevel
	lock       *sync.Mutex
}

// Info class, Contains all the info on what has to logged, time is the current time, Module is the specific module
// For which we are logging, level is the state, importance and type of message logged,
// Message contains the string to be logged, format is the format of string to be passed to sprintf
type Info struct {
	Id       uint64
	Time     string
	Module   string
	Level    LogLevel
	Line     int
	Filename string
	Message  string
	Category string
	//format   string
}

// Logger class that is an interface to user to log messages, Module is the module for which we are testing
// worker is variable of Worker class that is used in bottom layers to log the message
type Logger struct {
	Module      string
	worker      *Worker
	posOverride int
	WriteLock   *sync.Mutex
}

// init pkg
func init() {
	initColors()
	initFormatPlaceholders()
}

// Returns a proper string to be outputted for a particular info
func (r *Info) Output(format string) string {
	msg := fmt.Sprintf(format,
		r.Id,               // %[1] // %{id}
		r.Time,             // %[2] // %{time[:fmt]}
		r.Module,           // %[3] // %{module}
		r.Filename,         // %[4] // %{filename}
		r.Line,             // %[5] // %{line}
		r.logLevelString(), // %[6] // %{level}
		r.Message,          // %[7] // %{message}
		r.Category,         // %[8] // %{category}
	)
	// Ignore printf errors if len(args) > len(verbs)
	if i := strings.LastIndex(msg, "%!(EXTRA"); i != -1 {
		return msg[:i]
	}
	return msg
}

// Analyze and represent format string as printf format string and time format
func parseFormat(format string) (msgfmt, timefmt string) {
	if len(format) < 10 /* (len of "%{message} */ {
		return defFmt, defTimeFmt
	}
	timefmt = defTimeFmt
	idx := strings.IndexRune(format, '%')
	for idx != -1 {
		msgfmt += format[:idx]
		format = format[idx:]
		if len(format) > 2 {
			if format[1] == '{' {
				// end of curr verb pos
				if jdx := strings.IndexRune(format, '}'); jdx != -1 {
					// next verb pos
					idx = strings.Index(format[1:], "%{")
					// incorrect verb found ("...%{wefwef ...") but after
					// this, new verb (maybe) exists ("...%{inv %{verb}...")
					if idx != -1 && idx < jdx {
						msgfmt += "%%"
						format = format[1:]
						continue
					}
					// get verb and arg
					verb, arg := ph2verb(format[:jdx+1])
					msgfmt += verb
					// check if verb is time
					// here you can handle args for other verbs
					if verb == `%[2]s` && arg != "" /* %{time} */ {
						timefmt = arg
					}
					format = format[jdx+1:]
				} else {
					format = format[1:]
				}
			} else {
				msgfmt += "%%"
				format = format[1:]
			}
		}
		idx = strings.IndexRune(format, '%')
	}
	msgfmt += format
	return
}

// translate format placeholder to printf verb and some argument of placeholder
// (now used only as time format)
func ph2verb(ph string) (verb string, arg string) {
	n := len(ph)
	if n < 4 {
		return ``, ``
	}
	if ph[0] != '%' || ph[1] != '{' || ph[n-1] != '}' {
		return ``, ``
	}
	idx := strings.IndexRune(ph, ':')
	if idx == -1 {
		return phfs[ph], ``
	}
	verb = phfs[ph[:idx]+"}"]
	arg = ph[idx+1 : n-1]
	return
}

// Returns an instance of worker class, prefix is the string attached to every log,
// flag determine the log params, color parameters verifies whether we need colored outputs or not
func NewWorker(prefix string, flag int, color int, out io.Writer, lock *sync.Mutex) *Worker {
	return &Worker{Minion: log.New(out, prefix, flag), Color: color, format: defFmt, timeFormat: defTimeFmt, lock: lock}
}

func SetDefaultFormat(format string) {
	defFmt, defTimeFmt = parseFormat(format)
}

func (w *Worker) SetFormat(format string) {
	w.format, w.timeFormat = parseFormat(format)
}

func (l *Logger) SetFormat(format string) {
	l.worker.SetFormat(format)
}

func (w *Worker) SetLogLevel(level LogLevel) {
	w.level = level
}

func (l *Logger) SetLogLevel(level LogLevel) {
	l.worker.level = level
}

// Function of Worker class to log a string based on level
func (w *Worker) Log(level LogLevel, calldepth int, info *Info) (int, error) {

	if w.level < level {
		return 0, nil
	}

	w.lock.Lock()
	defer w.lock.Unlock()

	if w.Color != 0 {
		buf := &bytes.Buffer{}
		buf.Write([]byte(colors[level]))
		buf.Write([]byte(info.Output(w.format)))
		buf.Write([]byte("\033[0m"))
		return buf.Len(), w.Minion.Output(calldepth+1, buf.String())
	} else {
		output := info.Output(w.format)
		buf := &bytes.Buffer{}
		buf.Write([]byte(output))
		return buf.Len(), w.Minion.Output(calldepth+1, output)
	}
}

// Returns a proper string to output for colored logging
func colorString(color int) string {
	return fmt.Sprintf("\033[%dm", int(color))
}

// Initializes the map of colors
func initColors() {
	colors = map[LogLevel]string{
		CriticalLevel: colorString(Magenta),
		ErrorLevel:    colorString(Red),
		WarningLevel:  colorString(Yellow),
		NoticeLevel:   colorString(Green),
		DebugLevel:    colorString(Cyan),
		InfoLevel:     colorString(White),
	}
}

// Initializes the map of placeholders
func initFormatPlaceholders() {
	phfs = map[string]string{
		"%{id}":       "%[1]d",
		"%{time}":     "%[2]s",
		"%{module}":   "%[3]s",
		"%{filename}": "%[4]s",
		"%{file}":     "%[4]s",
		"%{line}":     "%[5]d",
		"%{level}":    "%[6]s",
		"%{lvl}":      "%.3[6]s",
		"%{message}":  "%[7]s",
		"%{category}": "%[8]s",
	}
}

// Returns a new instance of logger class, module is the specific module for which we are logging
// , color defines whether the output is to be colored or not, out is instance of type io.Writer defaults
// to os.Stderr
func New(args ...interface{}) (*Logger, error) {
	//initColors()

	var module string = "DEFAULT"
	var color int = 1
	var out io.Writer = os.Stderr
	var level LogLevel = InfoLevel

	for _, arg := range args {
		switch t := arg.(type) {
		case string:
			module = t
		case int:
			color = t
		case io.Writer:
			out = t
		case LogLevel:
			level = t
		default:
			panic("logger: Unknown argument")
		}
	}
	lock := &sync.Mutex{}
	newWorker := NewWorker("", 0, color, out, lock)
	newWorker.SetLogLevel(level)
	return &Logger{Module: module, worker: newWorker, WriteLock: lock}, nil
}

func (l *Logger) Update(args ...interface{}) {
	var color int = 1
	var out io.Writer = os.Stderr
	var level LogLevel = InfoLevel

	for _, arg := range args {
		switch t := arg.(type) {
		case int:
			color = t
		case io.Writer:
			out = t
		case LogLevel:
			level = t
		default:
			panic("logger: Unknown argument")
		}
	}
	lock := &sync.Mutex{}
	newWorker := NewWorker("", 0, color, out, lock)
	newWorker.SetLogLevel(level)
	l.worker = newWorker
}

// The log commnand is the function available to user to log message, lvl specifies
// the degree of the messagethe user wants to log, message is the info user wants to log
func (l *Logger) Log(lvl LogLevel, category, message string) {
	l.log_internal(lvl, category, message, 2)
}

func (l *Logger) GetPosOverride() int {
	return l.posOverride
}

func (l *Logger) SetPosOverride(pos int) {
	l.posOverride = pos
}

func (l *Logger) log_internal(lvl LogLevel, category, message string, pos int) (int, error) {
	//var formatString string = "#%d %s [%s] %s:%d ▶ %.3s %s"
	if l.posOverride != -1 {
		pos = l.posOverride
		l.posOverride = -1
	}
	_, filename, line, _ := runtime.Caller(pos)
	filename = path.Base(filename)
	info := &Info{
		Id:       atomic.AddUint64(&logNo, 1),
		Time:     time.Now().Format(l.worker.timeFormat),
		Module:   l.Module,
		Level:    lvl,
		Message:  message,
		Filename: filename,
		Line:     line,
		Category: category,
		//format:   formatString,
	}
	return l.worker.Log(lvl, 2, info)
}

// Fatal is just like func l.Critical logger except that it is followed by exit to program
func (l *Logger) Fatal(category, message string) {
	l.log_internal(CriticalLevel, category, message, 2)
	os.Exit(1)
}

// FatalF is just like func l.CriticalF logger except that it is followed by exit to program
func (l *Logger) FatalF(category, format string, a ...interface{}) {
	l.log_internal(CriticalLevel, category, fmt.Sprintf(format, a...), 2)
	os.Exit(1)
}

// FatalF is just like func l.CriticalF logger except that it is followed by exit to program
func (l *Logger) Fatalf(category, format string, a ...interface{}) {
	l.log_internal(CriticalLevel, category, fmt.Sprintf(format, a...), 2)
	os.Exit(1)
}

// Panic is just like func l.Critical except that it is followed by a call to panic
func (l *Logger) Panic(category, message string) {
	l.log_internal(CriticalLevel, category, message, 2)
	panic(message)
}

// PanicF is just like func l.CriticalF except that it is followed by a call to panic
func (l *Logger) PanicF(category, format string, a ...interface{}) {
	l.log_internal(CriticalLevel, category, fmt.Sprintf(format, a...), 2)
	panic(fmt.Sprintf(format, a...))
}

// PanicF is just like func l.CriticalF except that it is followed by a call to panic
func (l *Logger) Panicf(category, format string, a ...interface{}) {
	l.log_internal(CriticalLevel, category, fmt.Sprintf(format, a...), 2)
	panic(fmt.Sprintf(format, a...))
}

// Critical logs a message at a Critical Level
func (l *Logger) Critical(category, message string) {
	l.log_internal(CriticalLevel, category, message, 2)
}

// CriticalF logs a message at Critical level using the same syntax and options as fmt.Printf
func (l *Logger) CriticalF(category, format string, a ...interface{}) {
	l.log_internal(CriticalLevel, category, fmt.Sprintf(format, a...), 2)
}

// CriticalF logs a message at Critical level using the same syntax and options as fmt.Printf
func (l *Logger) Criticalf(category, format string, a ...interface{}) {
	l.log_internal(CriticalLevel, category, fmt.Sprintf(format, a...), 2)
}

// Error logs a message at Error level
func (l *Logger) Error(category, message string) {
	l.log_internal(ErrorLevel, category, message, 2)
}

// ErrorF logs a message at Error level using the same syntax and options as fmt.Printf
func (l *Logger) ErrorF(category, format string, a ...interface{}) {
	l.log_internal(ErrorLevel, category, fmt.Sprintf(format, a...), 2)
}

// ErrorF logs a message at Error level using the same syntax and options as fmt.Printf
func (l *Logger) Errorf(category, format string, a ...interface{}) {
	l.log_internal(ErrorLevel, category, fmt.Sprintf(format, a...), 2)
}

// Warning logs a message at Warning level
func (l *Logger) Warning(category, message string) {
	l.log_internal(WarningLevel, category, message, 2)
}

// WarningF logs a message at Warning level using the same syntax and options as fmt.Printf
func (l *Logger) WarningF(category, format string, a ...interface{}) {
	l.log_internal(WarningLevel, category, fmt.Sprintf(format, a...), 2)
}

// WarningF logs a message at Warning level using the same syntax and options as fmt.Printf
func (l *Logger) Warningf(category, format string, a ...interface{}) {
	l.log_internal(WarningLevel, category, fmt.Sprintf(format, a...), 2)
}

// Notice logs a message at Notice level
func (l *Logger) Notice(category, message string) {
	l.log_internal(NoticeLevel, category, message, 2)
}

// NoticeF logs a message at Notice level using the same syntax and options as fmt.Printf
func (l *Logger) NoticeF(category, format string, a ...interface{}) {
	l.log_internal(NoticeLevel, category, fmt.Sprintf(format, a...), 2)
}

// NoticeF logs a message at Notice level using the same syntax and options as fmt.Printf
func (l *Logger) Noticef(category, format string, a ...interface{}) {
	l.log_internal(NoticeLevel, category, fmt.Sprintf(format, a...), 2)
}

// Info logs a message at Info level
func (l *Logger) Info(category, message string) {
	l.log_internal(InfoLevel, category, message, 2)
}

// InfoF logs a message at Info level using the same syntax and options as fmt.Printf
func (l *Logger) InfoF(category, format string, a ...interface{}) {
	l.log_internal(InfoLevel, category, fmt.Sprintf(format, a...), 2)
}

// InfoF logs a message at Info level using the same syntax and options as fmt.Printf
func (l *Logger) Infof(category, format string, a ...interface{}) {
	l.log_internal(InfoLevel, category, fmt.Sprintf(format, a...), 2)
}

// Debug logs a message at Debug level
func (l *Logger) Debug(category, message string) {
	l.log_internal(DebugLevel, category, message, 2)
}

// DebugF logs a message at Debug level using the same syntax and options as fmt.Printf
func (l *Logger) DebugF(category, format string, a ...interface{}) {
	l.log_internal(DebugLevel, category, fmt.Sprintf(format, a...), 2)
}

// DebugF logs a message at Debug level using the same syntax and options as fmt.Printf
func (l *Logger) Debugf(category, format string, a ...interface{}) {
	l.log_internal(DebugLevel, category, fmt.Sprintf(format, a...), 2)
}

// Prints this goroutine's execution stack as an error with an optional message at the begining
func (l *Logger) StackAsError(category, message string) {
	if message == "" {
		message = "Stack info"
	}
	message += "\n"
	stack := Stack()
	stackParts := strings.Split(stack, "\n")
	newStackParts := []string{stackParts[0]}
	newStackParts = append(newStackParts, stackParts[3:]...)
	stack = strings.Join(newStackParts, "\n")
	l.log_internal(ErrorLevel, category, message+stack, 2)
}

// Prints this goroutine's execution stack as critical with an optional message at the begining
func (l *Logger) StackAsCritical(category, message string) {
	if message == "" {
		message = "Stack info"
	}
	message += "\n"
	stack := Stack()
	stackParts := strings.Split(stack, "\n")
	newStackParts := []string{stackParts[0]}
	newStackParts = append(newStackParts, stackParts[3:]...)
	stack = strings.Join(newStackParts, "\n")
	l.log_internal(CriticalLevel, category, message+stack, 2)
}

// Returns a string with the execution stack for this goroutine
func Stack() string {
	buf := make([]byte, 1000000)
	runtime.Stack(buf, false)
	buf = bytes.Trim(buf, "\x00")
	stack := string(buf)
	stackParts := strings.Split(stack, "\n")
	newStackParts := []string{stackParts[0]}
	newStackParts = append(newStackParts, stackParts[3:]...)
	stack = strings.Join(newStackParts, "\n")
	return stack
}

// Returns the loglevel as string
func (info *Info) logLevelString() string {
	logLevels := [...]string{
		"CRITICAL",
		"ERROR",
		"WARNING",
		"NOTICE",
		"INFO",
		"DEBUG",
	}
	return logLevels[info.Level-1]
}

func (l *Logger) Write(bytes []byte) (int, error) {
	_, err := l.log_internal(InfoLevel, "default_logger", string(bytes), 4)
	return len(bytes), err
}

func (l *Logger) Print(v ...interface{}) {
	_, _ = l.log_internal(InfoLevel, "default_logger", fmt.Sprint(v...), 4)
}
