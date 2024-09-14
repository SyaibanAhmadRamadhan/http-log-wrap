package httplogwrap

import (
	"context"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"net/http"
	"strings"
)

const SpanIDKey = "span_id"
const CorrelationIDKey = "correlation_id"

type OptHttpOtel struct {
	SetRequestIDHeader bool
	ExtraHeaders       []string
}

type OptHttpOtelFunc func(*OptHttpOtel)

func WithOutSetRequestIDHeader() OptHttpOtelFunc {
	return func(opt *OptHttpOtel) {
		opt.SetRequestIDHeader = false
	}
}

func WithExtraHeaders(headers ...string) OptHttpOtelFunc {
	return func(opt *OptHttpOtel) {
		opt.ExtraHeaders = headers
	}
}

func HttpOtel(next http.Handler, opts ...OptHttpOtelFunc) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		ctx, span := otel.Tracer("starting otel trace").Start(r.Context(), r.Method+" "+r.URL.Path, trace.WithAttributes(
			attribute.String("http.url", r.URL.String()),
			attribute.String("http.host", r.Host),
			attribute.String("http.client_ip", r.RemoteAddr),
			attribute.String("http.target", r.URL.Path),
			attribute.String("http.request.method", r.Method),
			attribute.String("http.request.user_agent", r.UserAgent()),
			attribute.String("http.request.content_type", r.Header.Get("Content-Type")),
			attribute.Int64("http.request.content_length", r.ContentLength),
			attribute.String("http.request.header.referer", r.Header.Get("Referer")),
			attribute.String("http.request.header.cookie", r.Header.Get("Cookie")),
		))
		defer span.End()

		if r.Header.Get("X-Correlation-ID") != "" {
			span.SetAttributes(attribute.String("http.request.header.x_correlation_id", r.Header.Get("X-Correlation-ID")))
			span.SetAttributes(attribute.String("correlation_id", r.Header.Get("X-Correlation-ID")))
			ctx = context.WithValue(ctx, CorrelationIDKey, r.Header.Get("X-Correlation-ID"))
			r = r.WithContext(ctx)
		}

		option := &OptHttpOtel{
			SetRequestIDHeader: true,
			ExtraHeaders:       make([]string, 0),
		}

		for _, opt := range opts {
			opt(option)
		}

		for _, v := range option.ExtraHeaders {
			extraHeader := r.Header.Get(v)
			span.SetAttributes(attribute.String("http.request.header."+convertHeaderName(v), extraHeader))
		}

		ctx = context.WithValue(ctx, SpanIDKey, span.SpanContext().SpanID().String())
		if option.SetRequestIDHeader {
			w.Header().Set("X-Request-ID", span.SpanContext().SpanID().String())
		}

		r = r.WithContext(ctx)
		next.ServeHTTP(w, r)
	}

	return http.HandlerFunc(fn)
}

func convertHeaderName(headerName string) string {
	headerName = strings.ToLower(headerName)

	result := strings.ReplaceAll(headerName, "-", "_")

	return result
}
