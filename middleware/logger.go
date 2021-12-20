package middleware

import (
	"fmt"
	"github.com/xuji-cny/yee/logger"
	"io"
	"log"

	"github.com/xuji-cny/yee"
	"github.com/valyala/fasttemplate"
)

//LoggerConfig defines config of logger middleware
type (
	LoggerConfig struct {
		Format   string
		Level    uint8
		IsLogger bool
	}
)

// DefaultLoggerConfig is default config of logger middleware
var DefaultLoggerConfig = LoggerConfig{
	Format:   `"url":"${url}" "method":"${method}" "status":${status} "protocol":"${protocol}" "remote_ip":"${remote_ip}" "bytes_in": "${bytes_in} bytes" "bytes_out": "${bytes_out} bytes"`,
	Level:    3,
	IsLogger: true,
}

// Logger is default implementation of logger middleware
func Logger() yee.HandlerFunc {
	return LoggerWithConfig(DefaultLoggerConfig)
}

// LoggerWithConfig is custom implementation of logger middleware
func LoggerWithConfig(config LoggerConfig) yee.HandlerFunc {
	if config.Format == "" {
		config.Format = DefaultLoggerConfig.Format
	}

	if config.Level == 0 {
		config.Level = DefaultLoggerConfig.Level
	}

	t, err := fasttemplate.NewTemplate(config.Format, "${", "}")

	if err != nil {
		log.Fatalf("unexpected error when parsing template: %s", err)
	}

	logger := logger.LogCreator()

	logger.SetLevel(config.Level)

	logger.IsLogger(config.IsLogger)

	return func(context yee.Context) (err error) {
		context.Next()
		s := t.ExecuteFuncString(func(w io.Writer, tag string) (int, error) {
			switch tag {
			case "url":
				p := context.Request().URL.Path
				if p == "" {
					p = "/"
				}
				return w.Write([]byte(p))
			case "method":
				return w.Write([]byte(context.Request().Method))
			case "status":
				return w.Write([]byte(fmt.Sprintf("%d", context.Response().Status())))
			case "remote_ip":
				return w.Write([]byte(context.RemoteIP()))
			case "host":
				return w.Write([]byte(context.Request().Host))
			case "protocol":
				return w.Write([]byte(context.Request().Proto))
			case "bytes_in":
				cl := context.Request().Header.Get(yee.HeaderContentLength)
				if cl == "" {
					cl = "0"
				}
				return w.Write([]byte(cl))
			case "bytes_out":
				return w.Write([]byte(fmt.Sprintf("%d", context.Response().Size())))
			default:
				return w.Write([]byte(""))
			}
		})
		if context.Response().Status() < 400 {
			logger.Info(s)
		} else {
			logger.Warn(s)
		}
		return
	}
}
