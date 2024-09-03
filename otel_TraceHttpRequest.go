package httplogwrap

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"io"
	"net/http"
	"time"
)

func TraceHttpOtel(next http.HandlerFunc, opts ...Option) http.HandlerFunc {

	return func(writer http.ResponseWriter, request *http.Request) {
		start := time.Now().UTC()

		recorder := &ResponseWriter{
			ResponseWriter: writer,
			logParams:      true,
			logRespBody:    true,
			logReqBody:     true,
		}

		for _, opt := range opts {
			opt(recorder)
		}

		if recorder.logParams {
			queryParamToSpan(request, request.URL.Query())
		}

		if recorder.logReqBody && (request.Method == http.MethodPost || request.Method == http.MethodPut) {
			_ = addRequestBodyToSpan(request)
		}

		next.ServeHTTP(recorder, request)
		duration := time.Since(start)

		_, span := otelTracer.Start(request.Context(), fmt.Sprintf("response body"),
			trace.WithAttributes(
				attribute.Int("http.response.status_code", recorder.status),
				attribute.String("http.response.size.format", formatSize(recorder.size)),
				attribute.Int("http.response.size.raw", recorder.size),
				attribute.String("http.response.duration", duration.String()),
				attribute.String("http.response.header.content_type", recorder.Header().Get("Content-Type")),
				attribute.String("http.response.header.cache_control", recorder.Header().Get("Cache-Control")),
			))
		if recorder.status == http.StatusOK {
			if recorder.logRespBody {
				span.SetAttributes(
					attribute.String("http.response.body", recorder.buffer.String()),
				)
			}
		}

		span.End()
	}
}

func queryParamToSpan(r *http.Request, attributes map[string][]string) {
	_, span := otelTracer.Start(r.Context(), "request query parameter")
	defer span.End()

	otelAttributes := make([]attribute.KeyValue, 0, len(attributes))
	for key, values := range attributes {
		for _, value := range values {
			otelAttributes = append(otelAttributes, attribute.String("http.request.query.params."+key, value))
		}
	}

	span.SetAttributes(attribute.String("http.request.query.raw", r.URL.RawQuery))
	span.SetAttributes(otelAttributes...)

	return
}

func addRequestBodyToSpan(r *http.Request) error {
	_, span := otelTracer.Start(r.Context(), "request body json")
	defer span.End()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	defer func() {
		errReqBody := r.Body.Close()
		if errReqBody != nil {
			span.RecordError(err)
		}
	}()

	var requestBody map[string]any
	if err = json.Unmarshal(body, &requestBody); err != nil {
		span.RecordError(err)
		return err
	}

	r.Body = io.NopCloser(bytes.NewBuffer(body))

	jsonString, err := json.Marshal(requestBody)
	if err != nil {
		span.RecordError(err)
		return err
	}

	span.SetAttributes(attribute.String("http.request.body.json", string(jsonString)))

	return nil
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

func GetCorrelationID(ctx context.Context) string {
	correlationID, ok := ctx.Value(CorrelationIDKey).(string)
	if !ok || correlationID == "" {
		return uuid.New().String()
	}

	return correlationID
}
