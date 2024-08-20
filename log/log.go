// @Version : 1.0
// @Author  : steven.wong
// @Email   : 'wangxk1991@gamil.com'
// @Time    : 2024/01/19 10:29:37
// Desc     :
// initLogging initializes the logger

package log

import (
	"context"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/logrusorgru/aurora"
	"github.com/piaobeizu/titan/hooks"
	"github.com/shiena/ansicolor"
	"github.com/sirupsen/logrus"
)

var Colorize = aurora.NewAurora(false)

func InitCliLog(ctx *context.Context, mode string) error {
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

func logLevelFromCtx(ctx *context.Context, defaultLevel logrus.Level, mode string) logrus.Level {
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

func screenLoggerHook(lvl logrus.Level) *hooks.Loghook {
	l := &hooks.Loghook{
		Skip: 5,
		Formatter: &hooks.ApolloFormatter{
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
	lf, err := hooks.LogFile(fmt.Sprintf("app-%s.%s.log", logName, time.Now().Local().Format("06-01-02")))
	if err != nil {
		return err
	}
	logrus.AddHook(fileLoggerHook(lf))
	return nil
}

func fileLoggerHook(logFile io.Writer) *hooks.Loghook {
	l := &hooks.Loghook{
		Skip: 5,
		Formatter: &hooks.ApolloFormatter{
			DisableColors:       true,
			MsgLength:           -1,
			ForceCutSpacialChar: false,
		},
		Writer: logFile,
	}

	l.SetLevel(logrus.DebugLevel)

	return l
}
