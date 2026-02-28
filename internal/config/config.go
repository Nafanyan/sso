package config

import (
	"flag"
	"os"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Env            string      `yaml:"env" env-default:"local"`
	StoragePath    string      `yaml:"storage_path" env-default:"/data/storage"`
	GRPC           GRPCConfig  `yaml:"grpc"`
	Redis          RedisConfig `yaml:"redis"`
	MigrationsPath string
	TokenTTL       time.Duration `yaml:"token_ttl" env-default:"1h"`
}

type GRPCConfig struct {
	Port    int32         `yaml:"port"`
	Timeout time.Duration `yaml:"timeout"`
}

type RedisConfig struct {
	Addr       string           `yaml:"addr" env-default:"localhost:6379"`
	Password   string           `yaml:"password" env-default:""`
	RateLimits RateLimitsConfig `yaml:"rate_limits"`
}

type RateLimitsConfig struct {
	LoginLimit  int64         `yaml:"login_limit" env-default:"5"`
	LoginWindow time.Duration `yaml:"login_window" env-default:"1m"`
}

func MustLoad() *Config {
	configPath := fetchConfigPath()
	if configPath == "" {
		panic("config path is empty")
	}

	return MustLoadPath(configPath)
}

func MustLoadPath(configPath string) *Config {
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		panic("config file does not exist: " + configPath)
	}

	var cfg Config

	if err := cleanenv.ReadConfig(configPath, &cfg); err != nil {
		panic("cannot read config: " + err.Error())
	}

	return &cfg
}

func fetchConfigPath() string {
	var res string

	flag.StringVar(&res, "config-path", "", "path to config file")
	flag.Parse()

	if res == "" {
		res = os.Getenv("CONFIG_PATH")
	}

	return res
}
