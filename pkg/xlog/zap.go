package xlog

import (
	"flag"
	"fmt"
	"os"
	"path"
	"runtime"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	Zap = zap.NewExample()

	EnvMode  = "development"
	EnvColor = false

	optsName       string
	optsLogPath    string
	optsServerHook func(p []byte)
	optsDebug      bool
)

func init() {
	mode := os.Getenv("XLOG_MODE")
	if mode != "" {
		EnvMode = mode
	}

	color := os.Getenv("XLOG_COLOR")
	if color == "" {
		if flag.Lookup("test.v") == nil {
			color = "true"
		} else {
			color = "false"
		}
	}
	EnvColor = color != "" && color != "false" && color != "0"
}

func Init(name string, logPath string, serverHook func(p []byte)) {
	if name == "" {
		name = "x"
	}
	if logPath == "" {
		logPath = path.Join("", "logs", name+".log")
	}

	optsName = name
	optsLogPath = logPath
	optsServerHook = serverHook
	optsDebug = EnvMode != "release"

	// Construct log
	Zap = NewZap(optsDebug)
	Zap.Info("zap init succeed", FileField())
}

func NewZap(debug bool) *zap.Logger {
	hook := lumberjack.Logger{
		Filename:   optsLogPath, // Log file path
		MaxSize:    128,         // The size of each log file in MB
		MaxAge:     30,          // The maximum number of days to retain a log file
		MaxBackups: 30,          // The maximum number of log file backups to retain
		Compress:   false,       // Whether to compress the log files
	}
	zapLogger = ZapLogger{ServerHook: optsServerHook, StdoutColor: EnvColor}

	encoderConfig := zapcore.EncoderConfig{
		MessageKey:     "msg",
		LevelKey:       "level",
		TimeKey:        "time",
		NameKey:        "logger",
		CallerKey:      "file",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.TimeEncoderOfLayout("2006-01-02T15:04:05.000Z07:00"),
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder, // Short path encoder
		EncodeName:     zapcore.FullNameEncoder,
	}

	atomicLevel := zap.NewAtomicLevel()
	writes := []zapcore.WriteSyncer{
		zapcore.AddSync(&hook),
		zapcore.AddSync(&zapLogger),
	}

	// Print to stdout
	zapLogger.SendToStdout = true

	if debug {
		atomicLevel.SetLevel(zap.DebugLevel)
	} else {
		atomicLevel.SetLevel(zap.InfoLevel)
	}

	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		zapcore.NewMultiWriteSyncer(writes...),
		atomicLevel,
	)

	// Enable development mode, stack trace
	// caller := zap.AddCaller()
	// Enable file and line number
	development := zap.Development()

	// Set initial fields
	field := zap.Fields(zap.String("app", optsName))

	// Construct log
	return zap.New(core, development, field)
}

func FileField() zap.Field {
	return zap.String("file", FileWithLineNum())
}

func FileWithLineNum() string {
	var (
		file string
		line int
	)

	for i := 0; i < 15; i++ {
		_, _file, _line, ok := runtime.Caller(i)
		if !ok {
			break
		}

		if !strings.Contains(_file, "/pkg/xlog/zap.go") &&
			!strings.Contains(_file, "/pkg/xlog/zap-logger.go") &&
			!strings.Contains(_file, "/pkg/xlog/xlog.go") &&
			!strings.Contains(_file, "/pkg/model/xgorm/") &&
			!strings.Contains(_file, "gin-gonic/gin") &&
			!strings.Contains(_file, "gorm.io/gorm") {

			file = _file
			line = _line
			break
		}
	}

	var (
		dir, fname string
	)
	ss := strings.Split(file, "/")
	if len(ss) > 0 {
		fname = ss[len(ss)-1]
	}
	if len(ss) > 1 {
		dir = ss[len(ss)-2]
	}

	return fmt.Sprintf("%s/%s:%d", dir, fname, line)
}
