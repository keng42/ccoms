package xlog

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type ZapLogger struct {
	SendToStdout bool
	StdoutColor  bool
	ServerHook   func(p []byte)
}

var zapLogger ZapLogger

func SetServerHook(hook func(p []byte)) {
	zapLogger.ServerHook = hook
}

func (l *ZapLogger) Write(p []byte) (n int, err error) {
	if !l.SendToStdout && l.ServerHook == nil {
		return
	}

	entry := map[string]interface{}{}
	err = json.Unmarshal(p, &entry)
	if err != nil {
		return
	}

	if l.SendToStdout {
		s := ""
		for k, v := range entry {
			if strings.HasPrefix(k, "x-") {
				s += k + ":" + fmt.Sprint(v) + " "
			}
		}
		if len(s) > 0 {
			s = "{ " + s + "}"
		}
		pre, sub := "", ""
		if l.StdoutColor {
			pre, sub = FixColor(fmt.Sprintf("%s", entry["level"]))
		}
		tStr := fmt.Sprintf("%s", entry["time"])
		t, err := time.Parse("2006-01-02T15:04:05.999Z07:00", tStr)
		if err == nil {
			tStr = t.Format("2006/01/02 15:04:05")
		}

		fname := ""
		f, ok := entry["file"]
		if ok {
			fname, _ = f.(string)
			// if ok {
			// 	ss := strings.Split(fname, "/")
			// 	if len(ss) > 0 {
			// 		fname = ss[len(ss)-1]
			// 	}
			// }
		}

		if len(fname) < 20 {
			fname = fname + strings.Repeat(" ", 20-len(fname))
		}
		if len(fname) > 20 {
			fname = fname[len(fname)-20:]
		}

		fmt.Printf(pre+"[%s] %s %s: %s %s"+sub+"\n", entry["app"], tStr, fname, entry["msg"], s)
	}

	// send to server
	if l.ServerHook != nil && entry["level"].(string) != "debug" {
		l.ServerHook(p)
	}

	return n, err
}
