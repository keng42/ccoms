package xlog_test

import (
	"ccoms/pkg/config"
	"ccoms/pkg/xlog"
	"path"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLog(t *testing.T) {
	xlog.Init("test", path.Join(config.DEVDATA, "logs/xlog-test.log"), nil)
	logger := xlog.GetLogger()
	require.NotNil(t, logger)

	logger.SetLevel("TRACE")

	logger.Trace("this is trace")
	logger.Debug("this is debug")
	logger.Info("this is info")
	logger.Warning("this is warning")
	logger.Error("this is error")
	logger.Fatal("this is fatal")
}
