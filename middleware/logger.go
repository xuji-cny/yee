package middleware

import (
	"fmt"
	"github.com/labstack/echo/v4"
	"github.com/valyala/fasttemplate"
	"io"
	"yee"
)

type (
	LoggerConfig struct {
		Format   string
		Level    uint8
		IsLogger bool
	}
)

var DefaultLoggerConfig = LoggerConfig{
	Format:   `"url":"${url}" "method":"${method}" "status":${status} "protocol":"${protocol}" "remote_ip":"${remote_ip}" "error":"${error}" "bytes_in": "${bytes_in} bytes" "bytes_out": "${bytes_out} bytes"`,
	Level:    3,
	IsLogger: true,
}

func Logger() yee.HandlerFunc {
	return LoggerWithConfig(DefaultLoggerConfig)
}

func LoggerWithConfig(config LoggerConfig) yee.HandlerFunc {
	if config.Format == "" {
		config.Format = DefaultLoggerConfig.Format
	}

	if config.Level == 0 {
		config.Level = DefaultLoggerConfig.Level
	}

	config.IsLogger = true

	t, err := fasttemplate.NewTemplate(config.Format, "${", "}")

	if err != nil {
		//logger.Error(fmt.Sprintf("unexpected error when parsing template: %s", err))
	}

	logger := yee.LogCreator()

	logger.SetLevel(config.Level)

	logger.IsLogger(config.IsLogger)

	return yee.HandlerFunc{
		Func: func(context yee.Context) (err error) {
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
					return w.Write([]byte(context.RemoteIp()))
				case "host":
					return w.Write([]byte(context.Request().Host))
				case "protocol":
					return w.Write([]byte(context.Request().Proto))
				case "bytes_in":
					cl := context.Request().Header.Get(echo.HeaderContentLength)
					if cl == "" {
						cl = "0"
					}
					return w.Write([]byte(cl))
				case "bytes_out":
					return w.Write([]byte(fmt.Sprintf("%d", context.Response().Size())))
				case "error":
					if context.Response().Status() != 200 {
						return w.Write(context.Response().Body())
					} else {
						return w.Write([]byte(""))
					}
				default:
					return w.Write([]byte(""))
				}
			})
			if context.Response().Status() == 200 {
				logger.Trace(s)
			} else {
				logger.Warn(s)
			}

			return
		},
		IsMiddleware: true,
	}
}
