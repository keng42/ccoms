package xetcd

import (
	"ccoms/pkg/xlog"
	"context"
	"errors"
	"strings"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

type Worker struct {
	Cli *clientv3.Client
}

var Shared *Worker
var logger = xlog.GetLogger()

func New(urls []string) (w *Worker, err error) {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   urls,
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return
	}

	w = &Worker{
		Cli: cli,
	}

	return
}

func InitShared(urls []string) (err error) {
	Shared, err = New(urls)
	if err != nil {
		return
	}

	// TODO ping and return error here if something wrong

	return
}

func SharedCli() *clientv3.Client {
	// TODO check connection
	return Shared.Cli
}

func Get(k string) (v string, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

	defer func() {
		if err != nil {
			logger.Errorf("xetcd Get k:%s failed with err:%s", k, err)
		} else {
			logger.Debugf("xetcd Get k:%s, v:%s", k, v)
		}
		cancel()
	}()

	cli := SharedCli()
	r, err := cli.Get(ctx, k)
	if err != nil {
		return
	}
	if r.Kvs == nil || r.Count == 0 {
		err = errors.New("not found")
		return
	}

	v = string(r.Kvs[0].Value)
	return
}

func Put(k string, v string) (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

	defer func() {
		if err != nil {
			logger.Errorf("xetcd Put k:%s, v:%s failed with err:%s", k, v, err)
		} else {
			logger.Debugf("xetcd Put k:%s, v:%s", k, v)
		}
		cancel()
	}()

	cli := SharedCli()
	_, err = cli.Put(ctx, k, v)
	if err != nil {
		return
	}

	return
}

func KeyBankService(coin string) string {
	return "bank_service_" + strings.ToLower(coin)
}

func KeyNatsService(coin string) string {
	return "nats_bank_" + strings.ToLower(coin)
}
