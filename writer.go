package whttp

import (
	"bytes"
	"go.opentelemetry.io/otel"
	"net/http"
)

var tracerName = "github.com/SyaibanAhmadRamadhan/http-log-wrap"
var otelTracer = otel.Tracer(tracerName)

type Option func(*ResponseWriter)

func WithLogRequestBody(log bool) Option {
	return func(e *ResponseWriter) {
		e.logReqBody = log
	}
}

func WithLogResponseBody(log bool) Option {
	return func(e *ResponseWriter) {
		e.logRespBody = log
	}
}

func WithLogParams(log bool) Option {
	return func(e *ResponseWriter) {
		e.logParams = log
	}
}

type ResponseWriter struct {
	http.ResponseWriter
	status      int
	size        int
	logParams   bool
	logRespBody bool
	logReqBody  bool
	buffer      *bytes.Buffer
}

func (rw *ResponseWriter) WriteHeader(status int) {
	rw.status = status
	rw.ResponseWriter.WriteHeader(status)
}

func (rw *ResponseWriter) Write(body []byte) (int, error) {
	if rw.status == 0 {
		rw.status = http.StatusOK
	}
	size, err := rw.ResponseWriter.Write(body)
	rw.size = size
	if rw.logRespBody {
		rw.buffer = new(bytes.Buffer)
		rw.buffer.Write(body)
	}
	return size, err
}
