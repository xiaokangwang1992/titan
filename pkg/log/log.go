/*
 @Version : 1.0
 @Author  : steven.wong
 @Email   : 'wwangxiaoakng@modelbest.cn'
 @Time    : 2024/04/09 15:42:04
 Desc     :
*/

package log

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/piaobeizu/titan/cache"
	"github.com/piaobeizu/titan/pkg/utils"
	"github.com/sirupsen/logrus"
)

const (
	red    = 31
	yellow = 33
	blue   = 36
	green  = 32
)

type LoggerFormatter struct {
	DisableColors       bool
	MsgLength           int
	ForceCutSpacialChar bool
}

func (m *LoggerFormatter) Format(entry *logrus.Entry) ([]byte, error) {
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
	timestamp := entry.Time.Local().Format("2006-01-02 15:04:05.000")
	msg = entry.Message
	if m.MsgLength != -1 && len(entry.Message) > m.MsgLength {
		msg = entry.Message[0:m.MsgLength]
	}
	if m.ForceCutSpacialChar {
		msg = strings.Replace(msg, "\n", "", -1)
	}

	linestr := ""
	lines := strings.Split(entry.Data["line"].(string), "/")
	linestr += strings.Join(lines[len(lines)-2:], "/")
	fields := []string{""}
	for k, v := range entry.Data {
		if k != "line" {
			fields = append(fields, fmt.Sprintf("%s:%v", k, v))
		}
	}
	sort.Strings(fields)
	fields[0] = linestr
	newLog += strings.Join(fields, "|")
	newLog = fmt.Sprintf("\x1b[%dm[%s] [%s] %s --- %s\n\x1b[0m",
		levelColor, levelMap[entry.Level.String()], timestamp, newLog, msg)
	if m.DisableColors {
		newLog = fmt.Sprintf("[%s] [%s] %s --- %s\n",
			levelMap[entry.Level.String()], timestamp, newLog, msg)
	}

	b.WriteString(newLog)
	return b.Bytes(), nil
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
	entry.Data["line"] = fmt.Sprintf("%s:%d", file, line)
	msg, err := h.Formatter.Format(entry)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to format log entry: %v", err)
		return err
	}
	_, err = h.Writer.Write(msg)
	return err
}

// 对caller进行递归查询, 直到找到非logrus包产生的第一个调用.
// 因为filename我获取到了上层目录名, 因此所有logrus包的调用的文件名都是 logrus/...
// 因此通过排除logrus开头的文件名, 就可以排除所有logrus包的自己的函数调用
func findCaller(skip int) (string, int) {
	file := ""
	line := 0
	for i := range 10 {
		file, line = getCaller(skip + i)
		if !strings.HasPrefix(file, "logrus") {
			break
		}
	}
	return file, line
	// return fmt.Sprintf("%s:%d", file, line)
}

// 这里其实可以获取函数名称的: fnName := runtime.FuncForPC(pc).Name()
// 但是我觉得有 文件名和行号就够定位问题, 因此忽略了caller返回的第一个值:pc
// 在标准库log里面我们可以选择记录文件的全路径或者文件名, 但是在使用过程成并发最合适的,
// 因为文件的全路径往往很长, 而文件名在多个包中往往有重复, 因此这里选择多取一层, 取到文件所在的上层目录那层.
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

func InitLog(app, logMode string) {
	level := logrus.InfoLevel
	if logMode == "" {
		logMode = utils.GetEnv("TITAN_DEBUG_MODE", "debug")
	}
	switch strings.ToLower(logMode) {
	case "debug":
		level = logrus.DebugLevel
	case "info":
		level = logrus.InfoLevel
	case "warn":
		level = logrus.WarnLevel
	case "error":
		level = logrus.ErrorLevel
	case "fatal":
		level = logrus.FatalLevel
	case "panic":
		level = logrus.PanicLevel
	}
	logrus.SetOutput(io.Discard)
	logrus.AddHook(loggerHook(level))

	if os.Getenv("LOG_FILE_PATH") != "" {
		lf, err := logFile(fmt.Sprintf("app-%s.%s.log", app, time.Now().Local().Format("06-01-02")))
		if err != nil {
			panic(err)
		}
		logrus.AddHook(fileLoggerHook(lf))
	}
}

func loggerHook(lvl logrus.Level) *Loghook {
	l := &Loghook{
		Skip: 5,
		Formatter: &LoggerFormatter{
			DisableColors:       false,
			MsgLength:           -1,
			ForceCutSpacialChar: false,
		},
	}
	l.Writer = os.Stdout
	l.SetLevel(lvl)

	return l
}

func fileLoggerHook(logFile io.Writer) *Loghook {
	l := &Loghook{
		Skip: 5,
		Formatter: &LoggerFormatter{
			DisableColors:       true,
			MsgLength:           -1,
			ForceCutSpacialChar: false,
		},
		Writer: logFile,
	}

	l.SetLevel(logrus.DebugLevel)

	return l
}

func logFile(file string) (io.Writer, error) {
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
