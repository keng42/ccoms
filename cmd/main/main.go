package main

import (
	"ccoms/pkg/bank"
	"ccoms/pkg/config"
	"ccoms/pkg/filedb"
	"ccoms/pkg/ingress"
	"ccoms/pkg/model"
	"ccoms/pkg/ome"
	"ccoms/pkg/xetcd"
	"ccoms/pkg/xlog"
	"ccoms/pkg/xnats"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/shopspring/decimal"
)

var logger = xlog.GetLogger()

var (
	fApp     string
	fCoin    string
	fSymbol  string
	fLogDir  string
	fLogFile string
)

var (
	apps = map[string]bool{"ingress": true, "bank": true, "ome": true, "bm": true, "fm": true}
)

func init() {
	flag.StringVar(&fApp, "app", "", "")
	flag.StringVar(&fCoin, "coin", "", "")
	flag.StringVar(&fSymbol, "symbol", "", "")
	flag.StringVar(&fLogDir, "logdir", "", "")
	flag.StringVar(&fLogFile, "logfile", "", "")
}

func main() {
	var err error
	flag.Parse()

	if !apps[fApp] {
		validApps := ""
		for k := range apps {
			validApps += k + ", "
		}
		panic("invalid app, only (" + validApps + ") avaliable")
	}

	// Initialize the Shared config
	config.EasyInit()

	// Initialize the logger
	if fLogDir == "" {
		fLogDir = filepath.Join(config.Shared.DataDir, "logs")
	}
	if fLogFile == "" {
		fLogFile = fApp + ".log"
	}
	logPath := filepath.Join(fLogDir, fLogFile)
	xlog.Init(fApp, logPath, nil)
	logger.Info(fApp + " started")
	logger.Infof("xlog in %s", logPath)

	// Handle signals
	go handleSignals()

	// Initialize the etcd instance
	err = xetcd.InitShared([]string{config.Shared.Etcd.Main.Url})
	if err != nil {
		logger.Errorf("xetcd.InitShared failed with err:%s", err)
		panic(err)
	}

	// Initialize the database instances(mysql, redis)
	// fatal if failed
	model.DBInit()

	// Start the app
	switch fApp {
	case "":
		return
	case "ingress":
		err = startIngress()
	case "bank":
		err = startBank()
	case "ome":
		err = startOme()
	case "bm":
		err = PrepareForBenchmark()
	case "fm":
		err = startFiledbMonitor()
	default:
		return
	}

	if err != nil {
		logger.Error(err)
		panic(err)
	}
}

// handleSignals handles linux signals
//
//	Function 1: Change log level via SIGUSR1 signal
//		docker exec <container_id> sh -c 'export XLOG_LVL=TRACE && kill -SIGUSR1 1'
func handleSignals() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGUSR1)
	logLevelChan := make(chan string)

	for {
		select {
		case sig := <-sigChan:
			if sig == syscall.SIGUSR1 {
				// Read log level from environment variable
				level := os.Getenv("XLOG_LVL")
				if level != "" {
					logLevelChan <- level
				}
			}
		case level := <-logLevelChan:
			logger := xlog.GetLogger()
			logger.SetLevel(level)
			logger.Infof("Log level set to %s via signal", level)
		}
	}
}

// dispatchBank dispatch bank according to order's symbol and side
func dispatchBank(symbol string, side int8) (bankCoin string) {
	ss := strings.Split(strings.ToUpper(symbol), "_")
	if len(ss) != 2 {
		return
	}
	base, quote := ss[0], ss[1]

	if side == model.OrderSideBid {
		bankCoin = quote
	} else if side == model.OrderSideAsk {
		bankCoin = base
	} else {
		return
	}

	return
}

// startIngress starts the ingress app
//
//	Function 1: Generate orders and send to Nats
//	Function 2: Benchmark the ingress app
func startIngress() (err error) {
	ing := &ingress.Worker{
		Nats: make(map[string]nats.JetStreamContext),
	}

	for i := 0; i < 100; i++ {
		_, err = ing.GetNats("BTC")
		if err != nil {
			logger.Errorf("ing.GetNats BTC failed with err:%s", err)
		} else {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err != nil {
		return
	}
	for i := 0; i < 100; i++ {
		_, err = ing.GetNats("USDT")
		if err != nil {
			logger.Errorf("ing.GetNats USDT failed with err:%s", err)
		} else {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err != nil {
		return
	}

	// create orders(ask and bid) with random price and quantity
	ch := make(chan xnats.OrderReq, 1024)
	ch2 := make(chan int64, 1024)
	curr := 16
	sentOds := int64(0)
	targetOds := int64(1_000_000)
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		for {
			num, ok := <-ch2
			if !ok {
				logger.Infof("comsumer:ch2 done")
				return
			}
			sentOds += num
			if sentOds >= targetOds {
				wg.Done()
			}
		}
	}()

	for i := 0; i < curr; i++ {
		go func(j int) {
			for {
				od, ok := <-ch
				if !ok {
					logger.Infof("comsumer:%d done", j)
					ch2 <- 1
					return
				}
				err := ing.SendOrderReq(dispatchBank(od.Symbol, od.Side), od)
				if err != nil {
					logger.Errorf("SendOrderReq failed with err:%s", err)
				}
				ch2 <- 1
			}
		}(i)
	}

	start := time.Now()
	for i := 0; i < int(targetOds); i++ {
		symbol := "BTC_USDT"
		price := 10 + rand.Int63n(100)
		qty := 1 + rand.Int63n(10)
		od := xnats.OrderReq{
			Symbol:   symbol,
			Owner:    1 + rand.Int63n(1000),
			Side:     int8(1 + rand.Int63n(2)),
			Type:     model.OrderTypeLimit,
			Price:    decimal.NewFromInt(price),
			Quantity: decimal.NewFromInt(qty),
			OrigQty:  decimal.NewFromInt(qty),
			Amount:   decimal.NewFromInt(price * qty),
			Time:     int64(1660000000 + i),
			FeeLevel: 0.01,
		}
		ch <- od
	}

	wg.Wait()

	// Benchmark result

	rate := int64(0)
	if int64(time.Since(start).Seconds()) > 0 {
		rate = sentOds / int64(time.Since(start).Seconds())
	}
	fmt.Printf(
		"Benchmark: Ingress sent %d orders to NATS in %s at %s with rate %d/sec\n",
		targetOds, time.Since(start), time.Now().Format(time.RFC3339), rate,
	)

	return
}

func startBank() (err error) {
	if fCoin == "" {
		return errors.New("empty coin")
	}

	bankw, err := bank.New(fCoin)
	if err != nil {
		return
	}

	err = bankw.Run()
	if err != nil {
		return
	}

	return
}

func startOme() (err error) {
	if fSymbol == "" {
		return errors.New("empty symbol")
	}
	omew, err := ome.New(fSymbol)
	if err != nil {
		return
	}

	err = omew.Run()
	if err != nil {
		return
	}

	return
}

// startFiledbMonitor starts the filedb monitor app
//
//	Function 1: Monitor the filedb log files and print the benchmark result every 30 seconds
func startFiledbMonitor() (err error) {
	for {
		time.Sleep(30 * time.Second)
		err = runFiledbMonitorOne()
		if err != nil {
			logger.Errorf("runFiledbMonitorOne failed with err:%s", err)
		}
	}
}

// runFiledbMonitorOne runs the filedb monitor one time
//
//	Function 1: Traverse all files ending with .log,
//		read the first and last line of each file,
//		each line should be a json object,
//		parse out {ts: nanosec, logID: int64} values,
//		calculate the time difference and logID difference, and output
func runFiledbMonitorOne() (err error) {
	filedbLogDir := path.Join(config.Shared.DataDir, "filedb")

	err = filepath.Walk(filedbLogDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && strings.HasSuffix(info.Name(), ".log") {
			fdb, err := filedb.New(path)
			if err != nil {
				return err
			}
			err = fdb.Open()
			if err != nil {
				return err
			}
			defer fdb.Close()

			firstLine, err := fdb.ReadFirstLine()
			if err != nil {
				return err
			}
			lastLine, err := fdb.ReadLastLine()
			if err != nil {
				return err
			}

			var firstLog, lastLog struct {
				Ts    int64 `json:"ts"`
				LogID int64 `json:"logID"`
			}

			if err := json.Unmarshal([]byte(firstLine), &firstLog); err != nil {
				return err
			}
			if err := json.Unmarshal([]byte(lastLine), &lastLog); err != nil {
				return err
			}

			timeDiff := (lastLog.Ts - firstLog.Ts)
			logIDDiff := lastLog.LogID - firstLog.LogID

			// timeDiff to duration
			duration := time.Duration(timeDiff) * time.Nanosecond
			lastLogTime := time.Unix(0, lastLog.Ts)

			rate := int64(0)
			if int64(duration.Seconds()) > 0 {
				rate = logIDDiff / int64(duration.Seconds())
			}
			fmt.Printf(
				"Benchmark: %s saved %d logs to filedb in %s at %s with rate %d/sec\n",
				path, logIDDiff, duration, lastLogTime.Format(time.RFC3339), rate,
			)
		}
		return nil
	})
	if err != nil {
		return
	}

	return
}
