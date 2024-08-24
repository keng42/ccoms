package xlog

const (
	ColorSubfix = "\033[0m"

	ColorTrace   = "\033[0;36m"
	ColorDebug   = "\033[1;36m"
	ColorInfo    = "\033[1;34m"
	ColorWarning = "\033[1;33m"
	ColorError   = "\033[1;31m"
	ColorFatal   = "\033[1;31m"

	ColorWhite = "\033[1;37m"
)

func FixColor(level string) (string, string) {
	switch level {
	case "trace":
		return ColorTrace, ColorSubfix
	case "debug":
		return ColorDebug, ColorSubfix
	// case "info":
	// 	return ColorInfo, ColorSubfix
	case "warn", "warning":
		return ColorWarning, ColorSubfix
	case "error":
		return ColorError, ColorSubfix
	case "fatal":
		return ColorFatal, ColorSubfix
	default:
		return "", ""
	}
}
