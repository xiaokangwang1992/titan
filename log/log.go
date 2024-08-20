// @Version : 1.0
// @Author  : steven.wong
// @Email   : 'wangxk1991@gamil.com'
// @Time    : 2024/01/19 10:29:37
// Desc     :
// initLogging initializes the logger

package log

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"runtime"
	"strings"
	"time"

	"github.com/logrusorgru/aurora"
	"github.com/piaobeizu/titan/cache"
	"github.com/shiena/ansicolor"
	"github.com/sirupsen/logrus"
)

var Colorize = aurora.NewAurora(false)

func InitCliLog(ctx context.Context, mode string) error {
	if strings.ToLower(mode) == "debug" {
		logrus.SetLevel(logrus.DebugLevel)
	} else if strings.ToLower(mode) == "trace" {
		logrus.SetLevel(logrus.TraceLevel)
	} else {
		logrus.SetLevel(logrus.InfoLevel)
	}
	logrus.SetOutput(io.Discard)
	initScreenLogger(logLevelFromCtx(ctx, logrus.InfoLevel, mode))
	return initFileLogger("cli")
}

func logLevelFromCtx(ctx context.Context, defaultLevel logrus.Level, mode string) logrus.Level {
	if strings.ToLower(mode) == "debug" {
		return logrus.DebugLevel
	} else if strings.ToLower(mode) == "trace" {
		return logrus.TraceLevel
	} else {
		return defaultLevel
	}
}

func initScreenLogger(lvl logrus.Level) {
	logrus.AddHook(screenLoggerHook(lvl))
}

func screenLoggerHook(lvl logrus.Level) *Loghook {
	l := &Loghook{
		Skip: 5,
		Formatter: &LogFormatter{
			DisableColors:       false,
			MsgLength:           -1,
			ForceCutSpacialChar: false,
		},
	}

	if runtime.GOOS == "windows" {
		l.Writer = ansicolor.NewAnsiColorWriter(os.Stdout)
	} else {
		l.Writer = os.Stdout
	}
	l.SetLevel(lvl)

	return l
}

func initFileLogger(logName string) error {
	lf, err := LogFile(fmt.Sprintf("app-%s.%s.log", logName, time.Now().Local().Format("06-01-02")))
	if err != nil {
		return err
	}
	logrus.AddHook(fileLoggerHook(lf))
	return nil
}

func fileLoggerHook(logFile io.Writer) *Loghook {
	l := &Loghook{
		Skip: 5,
		Formatter: &LogFormatter{
			DisableColors:       true,
			MsgLength:           -1,
			ForceCutSpacialChar: false,
		},
		Writer: logFile,
	}

	l.SetLevel(logrus.DebugLevel)

	return l
}

const (
	red    = 31
	yellow = 33
	blue   = 36
	green  = 32
)

type LogFormatter struct {
	DisableColors       bool
	MsgLength           int
	ForceCutSpacialChar bool
}

func (m *LogFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	var (
		b          *bytes.Buffer
		newLog     string
		levelColor int
		msg        string
		levelMap   = map[string]string{
			"info":    "INFOO",
			"error":   "ERROR",
			"warning": "WARNN",
			"debug":   "DEBUG",
			"panic":   "PANIC",
			"fatal":   "FATAL",
			"trace":   "TRACE",
		}
	)
	switch entry.Level {
	case logrus.DebugLevel, logrus.TraceLevel:
		levelColor = blue
	case logrus.WarnLevel:
		levelColor = yellow
	case logrus.ErrorLevel, logrus.FatalLevel, logrus.PanicLevel:
		levelColor = red
	case logrus.InfoLevel:
		levelColor = green
	default:
		levelColor = green
	}
	if entry.Buffer != nil {
		b = entry.Buffer
	} else {
		b = &bytes.Buffer{}
	}
	timestamp := entry.Time.Format("2006-01-02 15:04:05")
	msg = entry.Message
	if m.MsgLength != -1 && len(entry.Message) > m.MsgLength {
		msg = entry.Message[0:m.MsgLength]
	}
	if m.ForceCutSpacialChar {
		msg = strings.Replace(msg, "\n", "", -1)
	}
	lines := strings.Split(entry.Data["line"].(string), "/")
	entry.Data["line"] = strings.Join(lines[len(lines)-2:], "/")
	newLog = fmt.Sprintf("\x1b[%dm[%s] [%s] %-25s --- %s\x1b[0m\n",
		levelColor, levelMap[entry.Level.String()], timestamp, entry.Data["line"], msg)
	if m.DisableColors {
		newLog = fmt.Sprintf("[%s] [%s] %s --- %s\n",
			levelMap[entry.Level.String()], timestamp, entry.Data["line"], msg)
	}
	b.WriteString(newLog)
	return b.Bytes(), nil
}

func LogFile(file string) (io.Writer, error) {
	logDir := cache.Dir()
	if err := cache.EnsureDir(logDir); err != nil {
		return nil, fmt.Errorf("error while creating log directory %s: %s", logDir, err.Error())
	}

	fn := path.Join(logDir, file)
	logFile, err := os.OpenFile(fn, os.O_RDWR|os.O_CREATE|os.O_APPEND|os.O_SYNC, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to open log %s: %s", fn, err.Error())
	}

	_, _ = fmt.Fprintf(logFile, "\n[INFOO] [%s] - \"###### New session ######\"\n", time.Now().Local().Format("2006-01-02 15:04:05"))

	return logFile, nil
}

type Loghook struct {
	Skip      int
	Writer    io.Writer
	Formatter logrus.Formatter

	levels []logrus.Level
}

func (h *Loghook) SetLevel(level logrus.Level) {
	h.levels = []logrus.Level{}
	for _, l := range logrus.AllLevels {
		if level >= l {
			h.levels = append(h.levels, l)
		}
	}
}

func (h *Loghook) Levels() []logrus.Level {
	return h.levels
}

// Fire implement fire
func (h *Loghook) Fire(entry *logrus.Entry) error {
	file, line := findCaller(h.Skip)
	// if !entry.HasCaller() {
	// 	entry.Caller = &runtime.Frame{
	// 		File: file,
	// 		Line: line,
	// 	}
	// 	entry.Logger.ReportCaller = true
	// }
	// fmt.Printf("file,line is %s,%d\n", entry.Caller.File, entry.Caller.Line)
	// l := len(fmt.Sprintf("%s:%d", file, line)) + 2
	// entry.Data["line"] = fmt.Sprintf("%s%"+string(rune(l))+"s", fmt.Sprintf("%s:%d", file, line), "")[:l]
	entry.Data["line"] = fmt.Sprintf("%s:%d", file, line)
	msg, err := h.Formatter.Format(entry)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to format log entry: %v", err)
		return err
	}
	_, err = h.Writer.Write(msg)
	return err
}

func findCaller(skip int) (string, int) {
	file := ""
	line := 0
	for i := 0; i < 10; i++ {
		file, line = getCaller(skip + i)
		if !strings.HasPrefix(file, "logrus") {
			break
		}
	}
	return file, line
	// return fmt.Sprintf("%s:%d", file, line)
}

func getCaller(skip int) (string, int) {
	_, file, line, ok := runtime.Caller(skip)
	if !ok {
		return "", 0
	}
	n := 0
	for i := len(file) - 1; i > 0; i-- {
		if file[i] == '/' {
			n++
			if n >= 2 {
				file = file[i+1:]
				break
			}
		}
	}
	return file, line
}
