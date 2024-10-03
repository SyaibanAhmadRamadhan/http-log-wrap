package whttp

import (
	"context"
	"fmt"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"net/http"
	"net/url"
	"runtime"
	"strings"
)

func getMsgError(msg []string, code int) string {
	if len(msg) > 0 {
		return strings.Join(msg, ". ")
	}
	return http.StatusText(code)
}

func StackTrace(err error) error {
	pc := make([]uintptr, 15)
	n := runtime.Callers(2, pc)
	frames := runtime.CallersFrames(pc[:n])
	frame, _ := frames.Next()
	return fmt.Errorf("%s:%d: %w", frame.Function, frame.Line, err)
}

func RecordErrorOtel(ctx context.Context, err error) {
	span := trace.SpanFromContext(ctx)
	if !span.IsRecording() {
		return
	}
	
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
}

func queryParamToSpan(r *http.Request, span trace.Span) {
	otelAttributes := make([]attribute.KeyValue, 0, len(r.URL.Query()))
	for key, values := range r.URL.Query() {
		for _, value := range values {
			otelAttributes = append(otelAttributes, attribute.String("http.request.query.params."+key, value))
		}
	}

	span.SetAttributes(attribute.String("http.request.query.raw", r.URL.RawQuery))
	span.SetAttributes(otelAttributes...)

	return
}

func formatSize(size int) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)

	switch {
	case size >= GB:
		return fmt.Sprintf("%.2f GB", float64(size)/GB)
	case size >= MB:
		return fmt.Sprintf("%.2f MB", float64(size)/MB)
	case size >= KB:
		return fmt.Sprintf("%.2f KB", float64(size)/KB)
	default:
		return fmt.Sprintf("%d B", size)
	}
}

func GetTraceParent(ctx context.Context) string {
	traceParent, ok := ctx.Value(TraceParent).(string)
	if !ok || traceParent == "" {
		return ""
	}

	return traceParent
}

func convertHeaderName(headerName string) string {
	headerName = strings.ToLower(headerName)

	return headerName
}

func setAttr(ctx context.Context, attr attribute.KeyValue) {
	span := trace.SpanFromContext(ctx)
	if !span.IsRecording() {
		return
	}
	span.SetAttributes(attr)
}

func ParseQueryParam(r *http.Request) error {
	if r.Form == nil {
		r.Form = make(url.Values)
	}

	var newValues url.Values
	if r.URL != nil {
		var err error
		newValues, err = url.ParseQuery(r.URL.RawQuery)
		if err != nil {
			return StackTrace(err)
		}
	} else {
		newValues = make(url.Values)
	}

	copyValues(r.Form, newValues)

	return nil
}

func copyValues(dst, src url.Values) url.Values {
	for k, vs := range src {
		dst[k] = append(dst[k], vs...)
	}

	return dst
}
