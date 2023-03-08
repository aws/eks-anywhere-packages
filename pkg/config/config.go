package config

import (
	"strconv"
	"time"
)

const (
	DEVELOPMENT = "development"
	HEAD        = "HEAD"
	EPOCH       = "0"
)

var config Config

var version = DEVELOPMENT
var gitHash = HEAD
var buildTime = EPOCH

type Config struct {
	BuildInfo BuildInfo
	Env       BuildInfo
}

type BuildInfo struct {
	Version   string
	GitHash   string
	BuildTime time.Time
}

func init() {
	config = GetConfig()
}

func GetGlobalConfig() Config {
	return config
}

func GetConfig() Config {
	btime, err := strconv.ParseInt(buildTime, 10, 64)
	if err != nil {
		btime = 0
	}

	return Config{
		BuildInfo: BuildInfo{
			Version:   version,
			GitHash:   gitHash,
			BuildTime: time.Unix(btime, 0),
		},
	}
}
