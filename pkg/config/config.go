package config

import (
	"flag"
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v2"
)

// Config structs

type Config struct {
	IsDebug bool `yaml:"is_debug"`

	DataDir string `yaml:"data_dir"`
	AESKey  string `yaml:"aes_key"`

	MySQL MySQL `yaml:"mysql"`
	Redis Redis `yaml:"redis"`
	Etcd  Etcd  `yaml:"etcd"`

	Env Env `yaml:"env"`

	Sentry Sentry `yaml:"sentry"`
}

type MySQL struct {
	Main MySQLServer `yaml:"main"`
}

type MySQLServer struct {
	Enabled      bool   `yaml:"enabled"`
	Host         string `yaml:"host"`
	Port         int    `yaml:"port"`
	User         string `yaml:"user"`
	Pass         string `yaml:"pass"`
	DB           string `yaml:"db"`
	MaxOpenConns int    `yaml:"max_open_conns"`
}

type Redis struct {
	Main RedisServer `yaml:"main"`
}

type RedisServer struct {
	Enabled bool   `yaml:"enabled"`
	Addr    string `yaml:"addr"`
	DB      int    `yaml:"db"`
	Pass    string `yaml:"pass"`
	Timeout int    `yaml:"timeout"`
}

type Etcd struct {
	Main EtcdServer `yaml:"main"`
}

type EtcdServer struct {
	Enable bool   `yaml:"enable"`
	Url    string `yaml:"url"`
}

type Env struct {
	XlogMode  string `yaml:"xlog_mode"`
	XlogColor bool   `yaml:"xlog_color"`
}

type Sentry struct {
	Enabled bool   `yaml:"enabled"`
	Dsn     string `yaml:"dsn"`
}

// Global variables

const DEVDATA = "/usr/local/ccoms/devdata"

var Shared *Config // single instance of the config

var (
	fConfig string // config file path
)

func init() {
	flag.StringVar(&fConfig, "config", "", "specify the config file")
}

// Initialize the Shared config with the given config file path
func Init(configFile string) {
	file, err := os.Open(configFile)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	decoder := yaml.NewDecoder(file)
	err = decoder.Decode(&Shared)
	if err != nil {
		panic(err)
	}
}

// Initialize the Shared config with the default config file path
func EasyInit() {
	fpath := fConfig
	if fpath == "" {
		fpath = "config/config.yml"
	}

	// if the config file does not exist, use the default config file path
	if _, err := os.Stat(fpath); os.IsNotExist(err) {
		fpath = DEVDATA + "/config.yml"
		printf(fmt.Sprintf("use config: %s (DEVDATA)", fpath))
	} else {
		printf(fmt.Sprintf("use config: %s", fpath))
	}

	// initialize the config
	Init(fpath)
}

// Print the given string to the standard output
func printf(s string) {
	fmt.Printf("%s %s\n", time.Now().Format("2006/01/02 15:04:05"), s)
}
