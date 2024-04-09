// Copyright (c) 2017-2022 Snowflake Computing Inc. All rights reserved.

package gosnowflake

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"os"
	"runtime"
	"sync"
)

var paramsMutex *sync.Mutex

// SnowflakeDriver is a context of Go Driver
type SnowflakeDriver struct{}

// Open creates a new connection.
func (d SnowflakeDriver) Open(dsn string) (driver.Conn, error) {
	logger.Info("Open")
	ctx := context.Background()
	cfg, err := ParseDSN(dsn)
	if err != nil {
		return nil, err
	}
	return d.OpenWithConfig(ctx, *cfg)
}

// OpenWithConfig creates a new connection with the given Config.
func (d SnowflakeDriver) OpenWithConfig(ctx context.Context, config Config) (driver.Conn, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	if config.Tracing != "" {
		logger.SetLogLevel(config.Tracing)
	}
	logger.Info("OpenWithConfig")
	sc, err := buildSnowflakeConn(ctx, config)
	if err != nil {
		return nil, err
	}

	if err = authenticateWithConfig(sc); err != nil {
		return nil, err
	}
	sc.connectionTelemetry(&config)

	sc.startHeartBeat()
	sc.internal = &httpClient{sr: sc.rest}
	return sc, nil
}

func runningOnGithubAction() bool {
	return os.Getenv("GITHUB_ACTIONS") != ""
}

func skipRegisteration() bool {
	return os.Getenv("GOSNOWFLAKE_SKIP_REGISTERATION") != ""
}

var logger = CreateDefaultLogger()

func init() {
	if runtime.GOOS == "linux" {
		// TODO: delete this once we replaced 99designs/keyring (SNOW-1017659) and/or keyring#103 is resolved
		leak, logMsg := canDbusLeakProcesses()
		if leak {
			// 99designs/keyring#103 -> gosnowflake#773
			logger.Warn(logMsg)
		}
	}
	if !skipRegisteration() {
		sql.Register("snowflake", &SnowflakeDriver{})
	}
	logger.SetLogLevel("error")
	if runningOnGithubAction() {
		logger.SetLogLevel("fatal")
	}
	paramsMutex = &sync.Mutex{}
}
