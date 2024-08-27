package httplogwrap

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"io"
	"net/http"
	"strconv"
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
		duration := time.Since(start).Microseconds()

		_, span := otelTracer.Start(request.Context(), fmt.Sprintf("response body"),
			trace.WithAttributes(
				attribute.String("http.response.status", strconv.Itoa(recorder.status)),
				attribute.String("http.response.size", formatSize(recorder.size)),
				attribute.String("http.response.duration_ms", strconv.FormatInt(duration, 10)),
			))
		if recorder.status == http.StatusOK {
			if recorder.logRespBody {
				span.SetAttributes(
					attribute.String("response.body", recorder.buffer.String()),
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
			otelAttributes = append(otelAttributes, attribute.String("http.request.query_params."+key, value))
		}
	}

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
	if size < 1024 {
		return fmt.Sprintf("%d bytes", size)
	} else if size < 1024*1024 {
		return fmt.Sprintf("%.2f KB", float64(size)/1024)
	} else if size < 1024*1024*1024 {
		return fmt.Sprintf("%.2f MB", float64(size)/(1024*1024))
	} else {
		return fmt.Sprintf("%.2f GB", float64(size)/(1024*1024*1024))
	}
}
