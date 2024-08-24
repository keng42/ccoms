// Package xgorm is a copy of gorm.logger, with a few modifications to output better logs.
package xgorm

import (
	"ccoms/pkg/xlog"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"time"

	gl "gorm.io/gorm/logger"
)

var ErrRecordNotFound = gl.ErrRecordNotFound

// Colors
const (
	Reset       = gl.Reset
	Red         = gl.Red
	Green       = gl.Green
	Yellow      = gl.Yellow
	Blue        = gl.Blue
	Magenta     = gl.Magenta
	Cyan        = gl.Cyan
	White       = gl.White
	BlueBold    = gl.BlueBold
	MagentaBold = gl.MagentaBold
	RedBold     = gl.RedBold
	YellowBold  = gl.YellowBold
)

const (
	Silent = gl.Silent
	Error  = gl.Error
	Warn   = gl.Warn
	Info   = gl.Info
)

type LogLevel = gl.LogLevel

// Writer log writer interface
type Writer = gl.Writer

type Config = gl.Config

// Interface logger interface
type Interface = gl.Interface

var (
	Discard = New(log.New(ioutil.Discard, "", log.LstdFlags), Config{})
	Default = New(log.New(os.Stdout, "\r\n", log.LstdFlags), Config{
		SlowThreshold:             200 * time.Millisecond,
		LogLevel:                  Warn,
		IgnoreRecordNotFoundError: false,
		Colorful:                  true,
	})
	Recorder = traceRecorder{Interface: Default, BeginAt: time.Now()}

	zapLogger = xlog.GetLogger()
)

func New(writer Writer, config Config) Interface {
	var (
		infoStr      = "%s\n[info] "
		warnStr      = "%s\n[warn] "
		errStr       = "%s\n[error] "
		traceStr     = "%s\n[%.3fms] [rows:%v] %s"
		traceWarnStr = "%s %s\n[%.3fms] [rows:%v] %s"
		traceErrStr  = "%s %s\n[%.3fms] [rows:%v] %s"
	)

	if config.Colorful {
		infoStr = Green + "%s\n" + Reset + Green + "[info] " + Reset
		warnStr = BlueBold + "%s\n" + Reset + Magenta + "[warn] " + Reset
		errStr = Magenta + "%s\n" + Reset + Red + "[error] " + Reset
		traceStr = Green + "%s" + Reset + Yellow + "[%.3fms] " + BlueBold + "[rows:%v]" + Reset + " %s"
		traceWarnStr = Green + "%s" + Yellow + "%s\n" + Reset + RedBold + "[%.3fms] " + Yellow + "[rows:%v]" + Magenta + " %s" + Reset
		traceErrStr = RedBold + "%s " + MagentaBold + "%s\n" + Reset + Yellow + "[%.3fms] " + BlueBold + "[rows:%v]" + Reset + " %s"
	}

	return &logger{
		Writer:       writer,
		Config:       config,
		infoStr:      infoStr,
		warnStr:      warnStr,
		errStr:       errStr,
		traceStr:     traceStr,
		traceWarnStr: traceWarnStr,
		traceErrStr:  traceErrStr,
	}
}

type logger struct {
	Writer
	Config
	infoStr, warnStr, errStr            string
	traceStr, traceErrStr, traceWarnStr string
}

// LogMode log mode
func (l *logger) LogMode(level LogLevel) Interface {
	newlogger := *l
	newlogger.LogLevel = level
	return &newlogger
}

// Info print info
func (l logger) Info(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= Info {
		zapLogger.Infof(l.infoStr+msg, append([]interface{}{FileWithLineNum()}, data...)...)
	}
}

// Warn print warn messages
func (l logger) Warn(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= Warn {
		zapLogger.Warningf(l.warnStr+msg, append([]interface{}{FileWithLineNum()}, data...)...)
	}
}

// Error print error messages
func (l logger) Error(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= Error {
		zapLogger.Errorf(l.errStr+msg, append([]interface{}{FileWithLineNum()}, data...)...)
	}
}

// Trace print sql message
func (l logger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {

	if l.LogLevel <= Silent {
		return
	}

	elapsed := time.Since(begin)
	switch {
	case err != nil && l.LogLevel >= Error && (!errors.Is(err, ErrRecordNotFound) || !l.IgnoreRecordNotFoundError):
		sql, rows := fc()
		if rows == -1 {
			zapLogger.Errorf(l.traceErrStr, FileWithLineNum(), err, float64(elapsed.Nanoseconds())/1e6, "-", sql)
		} else {
			zapLogger.Errorf(l.traceErrStr, FileWithLineNum(), err, float64(elapsed.Nanoseconds())/1e6, rows, sql)
		}
	case elapsed > l.SlowThreshold && l.SlowThreshold != 0 && l.LogLevel >= Warn:
		sql, rows := fc()
		slowLog := fmt.Sprintf("SLOW SQL >= %v", l.SlowThreshold)
		if rows == -1 {
			zapLogger.Warningf(l.traceWarnStr, FileWithLineNum(), slowLog, float64(elapsed.Nanoseconds())/1e6, "-", sql)
		} else {
			zapLogger.Warningf(l.traceWarnStr, FileWithLineNum(), slowLog, float64(elapsed.Nanoseconds())/1e6, rows, sql)
		}
	case l.LogLevel == Info:
		sql, rows := fc()
		if rows == -1 {
			zapLogger.Infof(l.traceStr, FileWithLineNum(), float64(elapsed.Nanoseconds())/1e6, "-", sql)
		} else {
			zapLogger.Infof(l.traceStr, FileWithLineNum(), float64(elapsed.Nanoseconds())/1e6, rows, sql)
		}
	}
}

type traceRecorder struct {
	Interface
	BeginAt      time.Time
	SQL          string
	RowsAffected int64
	Err          error
}

func (l traceRecorder) New() *traceRecorder {
	return &traceRecorder{Interface: l.Interface, BeginAt: time.Now()}
}

func (l *traceRecorder) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	l.BeginAt = begin
	l.SQL, l.RowsAffected = fc()
	l.Err = err
}

func FileWithLineNum() string {
	// the second caller usually from gorm internal, so set i start from 2
	// for i := 4; i < 15; i++ {
	// 	_, file, line, ok := runtime.Caller(i)
	// 	if ok && (!strings.Contains(file, "gorm.io/gorm") || strings.HasSuffix(file, "_test.go")) {
	// 		ss := strings.Split(file, "/")
	// 		if len(ss) > 0 {
	// 			return ss[len(ss)-1] + ":" + strconv.FormatInt(int64(line), 10)
	// 		}
	// 	}
	// }

	return ""
}
