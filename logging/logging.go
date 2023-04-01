package logging

import (
	"context"
	"fmt"
	"github.com/Dri0m/flashpoint-submission-system/config"
	"github.com/Dri0m/flashpoint-submission-system/utils"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/felixge/httpsnoop"
	"github.com/gemnasium/logrus-graylog-hook/v3"
	"github.com/sirupsen/logrus"
	"gopkg.in/natefinch/lumberjack.v2"
)

// InitLogger returns a configured logger
func InitLogger() *logrus.Logger {
	mw := io.MultiWriter(os.Stdout, &lumberjack.Logger{
		Filename:   "log.log",
		MaxSize:    500, // megabytes
		MaxAge:     0,   //days
		MaxBackups: 0,
		Compress:   true,
	})
	l := logrus.New()
	if config.EnvBool("GRAYLOG_ENABLED") {
		host := config.EnvString("GRAYLOG_HOST")
		env := config.EnvString("GRAYLOG_ENV")
		hook := graylog.NewGraylogHook(host, map[string]interface{}{"env": env})
		l.AddHook(hook)
	}
	l.SetFormatter(&logrus.TextFormatter{
		DisableColors:   true,
		FullTimestamp:   true,
		TimestampFormat: time.RFC3339Nano,
	})
	l.SetOutput(mw)
	l.SetLevel(logrus.TraceLevel)
	l.SetReportCaller(true)
	return l
}

// https://presstige.io/p/Logging-HTTP-requests-in-Go-233de7fe59a747078b35b82a1b035d36

// HTTPReqInfo is HTTP request info
type HTTPReqInfo struct {
	// GET etc.
	method  string
	uri     string
	referer string
	ipaddr  string

	code int

	size int64

	duration  time.Duration
	userAgent string
}

// LogRequestHandler is logging handler
func LogRequestHandler(l *logrus.Entry, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r = r.WithContext(context.WithValue(r.Context(), utils.CtxKeys.Log, l))
		r = r.WithContext(context.WithValue(r.Context(), utils.CtxKeys.RequestID, utils.NewRealRandomStringProvider().RandomString(16)))
		ri := &HTTPReqInfo{
			method:    r.Method,
			uri:       r.URL.String(),
			referer:   r.Header.Get("Referer"),
			userAgent: r.Header.Get("User-Agent"),
		}

		ri.ipaddr = requestGetRemoteAddress(r)

		// this runs handler h and captures information about
		// HTTP request
		m := httpsnoop.CaptureMetrics(h, w, r)

		ri.code = m.Code
		ri.size = m.Written
		ri.duration = m.Duration
		utils.LogCtx(r.Context()).WithFields(logrus.Fields{"method": ri.method, "ip": ri.ipaddr, "uri": ri.uri, "statusCode": ri.code, "size": ri.size, "duration_ns": fmt.Sprintf("%d", ri.duration.Nanoseconds()), "userAgent": ri.userAgent})
	})
}

// Request.RemoteAddress contains port, which we want to remove i.e.:
// "[::1]:58292" => "[::1]"
func ipAddrFromRemoteAddr(s string) string {
	idx := strings.LastIndex(s, ":")
	if idx == -1 {
		return s
	}
	return s[:idx]
}

// requestGetRemoteAddress returns ip address of the client making the request,
// taking into account http proxies
func requestGetRemoteAddress(r *http.Request) string {
	hdr := r.Header
	hdrRealIP := hdr.Get("X-Real-Ip")
	hdrForwardedFor := hdr.Get("X-Forwarded-For")
	if hdrRealIP == "" && hdrForwardedFor == "" {
		return ipAddrFromRemoteAddr(r.RemoteAddr)
	}
	if hdrForwardedFor != "" {
		// X-Forwarded-For is potentially a list of addresses separated with ","
		parts := strings.Split(hdrForwardedFor, ",")
		for i, p := range parts {
			parts[i] = strings.TrimSpace(p)
		}
		// TODO: should return first non-local address
		return parts[0]
	}
	return hdrRealIP
}
