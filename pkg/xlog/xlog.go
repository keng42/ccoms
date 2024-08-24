package xlog

import (
	"fmt"
	"os"
	"strings"
)

type Logger struct {
	level int
}

const (
	TRACE = iota
	DEBUG
	INFO
	WARNING
	ERROR
	FATAL
)

var levelNames = []string{
	"TRACE",
	"DEBUG",
	"INFO",
	"WARNING",
	"ERROR",
	"FATAL",
}

var _logger *Logger

func GetLogger() *Logger {
	if _logger != nil {
		return _logger
	}
	level := INFO
	lvl := strings.ToUpper(os.Getenv("XLOG_LVL"))
	switch lvl {
	case "T", "TRC", "TRACE":
		level = TRACE
	case "D", "DBG", "DEBUG":
		level = DEBUG
	case "I", "INF", "INFO":
		level = INFO
	case "W", "WRN", "WARNING":
		level = WARNING
	case "E", "ERR", "ERROR":
		level = ERROR
	case "F", "FTL", "FATAL":
		level = FATAL
	}
	_logger = &Logger{
		level: level,
	}
	_logger.Infof("using xlog with %s, XLOG_LVL:%s", levelNames[level], lvl)
	return _logger
}

func (s *Logger) SetLevel(level string) {
	for i, v := range levelNames {
		if v == level {
			s.level = i
			s.Infof("set xlog level to %s", level)
			return
		}
	}

	s.Infof("set xlog level to %s failed", level)
}

func (s *Logger) SetLevelNum(num int) {
	s.level = num
}

func (s *Logger) GetLevel() int {
	return s.level
}

func (s *Logger) Trace(args ...interface{}) {
	if TRACE >= s.level {
		Zap.Debug(fmt.Sprintf("[TRC] %v", argsToString(args)), FileField())
	}
}

func (s *Logger) Tracef(format string, args ...interface{}) {
	if TRACE >= s.level {
		Zap.Debug(fmt.Sprintf("[TRC] %v", fmt.Sprintf(format, args...)), FileField())
	}
}

func (s *Logger) Debug(args ...interface{}) {
	if DEBUG >= s.level {
		Zap.Debug(fmt.Sprintf("[DBG] %v", argsToString(args)), FileField())
	}
}

func (s *Logger) Debugf(format string, args ...interface{}) {
	if DEBUG >= s.level {
		Zap.Debug(fmt.Sprintf("[DBG] %v", fmt.Sprintf(format, args...)), FileField())
	}
}

func (s *Logger) Info(args ...interface{}) {
	if INFO >= s.level {
		Zap.Info(fmt.Sprintf("[INF] %v", argsToString(args)), FileField())
	}
}

func (s *Logger) Infof(format string, args ...interface{}) {
	if INFO >= s.level {
		Zap.Info(fmt.Sprintf("[INF] %v", fmt.Sprintf(format, args...)), FileField())
	}
}

func (s *Logger) Warning(args ...interface{}) {
	if WARNING >= s.level {
		Zap.Warn(fmt.Sprintf("[WRN] %v", argsToString(args)), FileField())
	}
}

func (s *Logger) Warningf(format string, args ...interface{}) {
	if WARNING >= s.level {
		Zap.Warn(fmt.Sprintf("[WRN] %v", fmt.Sprintf(format, args...)), FileField())
	}
}

func (s *Logger) Error(args ...interface{}) {
	if ERROR >= s.level {
		msg := fmt.Sprintf("[ERR] %v", argsToString(args))
		Zap.Error(msg, FileField())
		// for _, item := range args {
		// 	err, ok := item.(error)
		// 	if ok {
		// 		ev := mysentry.EventFromException(err)
		// 		ev.Message = msg
		// 		sentry.CaptureEvent(ev)
		// 	}
		// }
	}
}

func (s *Logger) Errorf(format string, args ...interface{}) {
	if ERROR >= s.level {
		msg := fmt.Sprintf("[ERR] %v", fmt.Sprintf(format, args...))
		Zap.Error(msg, FileField())
		// for _, item := range args {
		// 	err, ok := item.(error)
		// 	if ok {
		// 		ev := mysentry.EventFromException(err)
		// 		ev.Message = msg
		// 		sentry.CaptureEvent(ev)
		// 	}
		// }
	}
}

func (s *Logger) Fatal(args ...interface{}) {
	Zap.Fatal(fmt.Sprintf("[FTL] %v", argsToString(args)), FileField())
	os.Exit(1)
}

func (s *Logger) Fatalf(format string, args ...interface{}) {
	Zap.Fatal(fmt.Sprintf("[FTL] %v", fmt.Sprintf(format, args...)), FileField())
	os.Exit(1)
}

func (s *Logger) Write(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}
	Zap.Info(string(p), FileField())
	return 0, nil
}

func argsToString(args ...interface{}) string {
	s := fmt.Sprintf("%v", args...)
	if len(s) <= 2 {
		return s
	}
	return s[1 : len(s)-1]
}
