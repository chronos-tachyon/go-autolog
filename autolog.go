package autolog

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/mattn/go-isatty"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
	LogLevelVarName      = "LOG_LEVEL"
	LogColorVarName      = "LOG_COLOR"
	LogOutputVarName     = "LOG_OUTPUT"
	LogFormatVarName     = "LOG_FORMAT"
	LogTimeFormatVarName = "LOG_TIMEFORMAT"
)

var logTimeFormatMap = map[string]string{
	"kitchen":    "3:04PM",
	"kitchen.s":  "3:04:05PM",
	"kitchen.ms": "3:04:05.999PM",
	"kitchen.us": "3:04:05.999999PM",
	"kitchen.ns": "3:04:05.999999999PM",
	"rfc822":     "02 Jan 2006 15:04 -0700",
	"rfc822.s":   "02 Jan 2006 15:04:05 -0700",
	"rfc822.ms":  "02 Jan 2006 15:04:05.999 -0700",
	"rfc822.us":  "02 Jan 2006 15:04:05.999999 -0700",
	"rfc822.ns":  "02 Jan 2006 15:04:05.999999999 -0700",
	"rfc1123":    "Mon, 02 Jan 2006 15:04 -0700",
	"rfc1123.s":  "Mon, 02 Jan 2006 15:04:05 -0700",
	"rfc1123.ms": "Mon, 02 Jan 2006 15:04:05.999 -0700",
	"rfc1123.us": "Mon, 02 Jan 2006 15:04:05.999999 -0700",
	"rfc1123.ns": "Mon, 02 Jan 2006 15:04:05.999999999 -0700",
	"rfc3339":    "2006-01-02T15:04Z07:00",
	"rfc3339.s":  "2006-01-02T15:04:05Z07:00",
	"rfc3339.ms": "2006-01-02T15:04:05.999Z07:00",
	"rfc3339.us": "2006-01-02T15:04:05.999999Z07:00",
	"rfc3339.ns": "2006-01-02T15:04:05.999999999Z07:00",
	"iso8601":    "2006-01-02T15:04Z07:00",
	"iso8601.s":  "2006-01-02T15:04:05Z07:00",
	"iso8601.ms": "2006-01-02T15:04:05.999Z07:00",
	"iso8601.us": "2006-01-02T15:04:05.999999Z07:00",
	"iso8601.ns": "2006-01-02T15:04:05.999999999Z07:00",
}

func ExpandTimeFormat(str string) string {
	key := strings.ToLower(strings.ReplaceAll(str, "µ", "u"))
	if value, found := logTimeFormatMap[key]; found {
		return value
	}
	return str
}

func ExpandPath(str string, now time.Time) string {
	return Strftime(str, now)
}

var (
	gOnce      sync.Once
	gWriter    io.Writer
	gNeedClose bool
)

func Init() {
	gOnce.Do(func() {
		zerolog.TimeFieldFormat = zerolog.TimeFormatUnixMs
		zerolog.DurationFieldUnit = time.Second
		zerolog.DurationFieldInteger = false

		if str, found := os.LookupEnv(LogLevelVarName); found {
			level, err := zerolog.ParseLevel(str)
			if err != nil {
				panic(fmt.Errorf("%s: %w", LogLevelVarName, err))
			}
			zerolog.SetGlobalLevel(level)
		}

		logColor := triStateAuto
		if str, found := os.LookupEnv(LogColorVarName); found {
			if err := logColor.Parse(str); err != nil {
				panic(fmt.Errorf("%s: %w", LogColorVarName, err))
			}
		}

		logOutput := getenv(LogOutputVarName, "stderr")
		switch {
		case logOutput == "stdout":
			gWriter = os.Stdout

		case logOutput == "stderr":
			gWriter = os.Stderr

		case strings.HasPrefix(logOutput, "file:"):
			var err error
			gWriter, err = openFile(filepath.Clean(logOutput[5:]))
			if err != nil {
				panic(fmt.Errorf("%s: %w", LogOutputVarName, err))
			}
			gNeedClose = true

		case strings.HasPrefix(logOutput, "pattern:"):
			var err error
			gWriter, err = NewRotatingLogWriter(filepath.Clean(logOutput[8:]), true)
			if err != nil {
				panic(fmt.Errorf("%s: %w", LogOutputVarName, err))
			}
			gNeedClose = true

		default:
			panic(fmt.Errorf("%s: expected \"stdout\", \"stderr\", or \"file:<path>\"", LogOutputVarName))
		}

		defaultLogFormat := "json"
		if file, ok := gWriter.(*os.File); ok {
			switch {
			case isatty.IsTerminal(file.Fd()):
				defaultLogFormat = "console"
			case isatty.IsCygwinTerminal(file.Fd()):
				defaultLogFormat = "console"
			default:
				if logColor == triStateAuto {
					logColor = triStateNo
				}
			}
		}

		logFormat := getenv(LogFormatVarName, defaultLogFormat)
		var logWriter io.Writer
		var c *zerolog.ConsoleWriter
		switch logFormat {
		case "json":
			logWriter = gWriter
		case "console":
			c = &zerolog.ConsoleWriter{Out: gWriter, NoColor: logColor == triStateNo}
			logWriter = c
		default:
			panic(fmt.Errorf("%s: unknown log format %q; expected one of [\"console\", \"json\"]", LogFormatVarName, logFormat))
		}

		logTimeFormat, found := os.LookupEnv(LogTimeFormatVarName)
		if found {
			logTimeFormat = ExpandTimeFormat(logTimeFormat)
			if c == nil {
				zerolog.TimeFieldFormat = logTimeFormat
			} else {
				c.TimeFormat = logTimeFormat
			}
		}

		log.Logger = zerolog.New(logWriter).With().Timestamp().Logger()
		zerolog.DefaultContextLogger = &log.Logger
	})
}

func Writer() io.Writer {
	return gWriter
}

func Rotate() error {
	if x, ok := gWriter.(*RotatingLogWriter); ok {
		return x.Rotate()
	}
	return nil
}

func Done() error {
	if gNeedClose {
		return gWriter.(io.Closer).Close()
	}
	return nil
}

type RotatingLogWriter struct {
	mu        sync.RWMutex
	file      *os.File
	name      string
	pattern   string
	isPattern bool
}

func NewRotatingLogWriter(pattern string, isPattern bool) (*RotatingLogWriter, error) {
	name := pattern
	if isPattern {
		name = ExpandPath(name, time.Now())
	}

	file, err := openFile(name)
	if err != nil {
		return nil, err
	}

	w := &RotatingLogWriter{file: file, name: name, pattern: pattern, isPattern: isPattern}
	return w, nil
}

func (w *RotatingLogWriter) Write(p []byte) (int, error) {
	notNil(w)

	w.mu.RLock()
	defer w.mu.RUnlock()

	if w.file == nil {
		return 0, fs.ErrClosed
	}
	return w.file.Write(p)
}

func (w *RotatingLogWriter) Close() error {
	notNil(w)

	var name string
	var file *os.File

	w.mu.Lock()
	name, w.name = w.name, name
	file, w.file = w.file, file
	w.mu.Unlock()

	return closeFile(name, file)
}

func (w *RotatingLogWriter) Rotate() error {
	notNil(w)

	name := w.pattern
	if w.isPattern {
		name = ExpandPath(name, time.Now())
	}

	file, err := openFile(name)
	if err != nil {
		return err
	}

	w.mu.Lock()
	name, w.name = w.name, name
	file, w.file = w.file, file
	w.mu.Unlock()

	return closeFile(name, file)
}

func (w *RotatingLogWriter) WithFile(fn func(name string, file *os.File) error) error {
	notNil(w)
	w.mu.RLock()
	defer w.mu.RUnlock()
	return fn(w.name, w.file)
}

var (
	_ io.Writer = (*RotatingLogWriter)(nil)
	_ io.Closer = (*RotatingLogWriter)(nil)
)

type triState byte

const (
	triStateAuto triState = iota
	triStateYes
	triStateNo
)

var triStateNames = [...]string{"auto", "yes", "no"}
var triStateMap = map[string]triState{
	"":     triStateAuto,
	"auto": triStateAuto,

	"1":    triStateYes,
	"y":    triStateYes,
	"yes":  triStateYes,
	"t":    triStateYes,
	"true": triStateYes,
	"on":   triStateYes,

	"0":     triStateNo,
	"n":     triStateNo,
	"no":    triStateNo,
	"f":     triStateNo,
	"false": triStateNo,
	"off":   triStateNo,
}

func (enum triState) String() string {
	if enum < triState(len(triStateNames)) {
		return triStateNames[enum]
	}
	return triStateNames[0]
}

func (enum triState) MarshalText() ([]byte, error) {
	return []byte(enum.String()), nil
}

func (enum *triState) Parse(input string) error {
	*enum = 0

	if value, found := triStateMap[input]; found {
		*enum = value
		return nil
	}

	lc := strings.ToLower(input)
	if value, found := triStateMap[lc]; found {
		*enum = value
		return nil
	}

	return fmt.Errorf("unknown tri-state value %q", input)
}

func getenv(name string, defaultValue string) string {
	if value, found := os.LookupEnv(name); found {
		return value
	}
	return defaultValue
}

func openFile(name string) (*os.File, error) {
	file, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o666)
	if err != nil {
		return nil, fmt.Errorf("failed to open file for appending: %q: %w", name, err)
	}
	return file, nil
}

func closeFile(name string, file *os.File) error {
	if file == nil {
		return fs.ErrClosed
	}

	err := file.Sync()
	if err != nil {
		_ = file.Close()
		return fmt.Errorf("failed to sync file before closing: %q: %w", name, err)
	}

	err = file.Close()
	if err != nil {
		return fmt.Errorf("failed to close file: %q: %w", name, err)
	}

	return nil
}

func notNil[T any](ptr *T) {
	if ptr == nil {
		panic(fmt.Errorf("%T is nil", ptr))
	}
}
