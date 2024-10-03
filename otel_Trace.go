package whttp

import (
	"context"
	"encoding/json"
	errors "errors"
	"fmt"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-playground/validator/v10"
	"github.com/gorilla/schema"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"io"
	"net/http"
	"runtime/debug"
	"strings"
)

type OptHttpOtelFunc func(*opentelemetry)

func WithPropagator() OptHttpOtelFunc {
	return func(o *opentelemetry) {
		o.propagators = otel.GetTextMapPropagator()
	}
}

func WithRecoverMode(logStdOutPanic bool) OptHttpOtelFunc {
	return func(o *opentelemetry) {
		o.logStdOutPanic = logStdOutPanic
		o.recover = true
	}
}

type opentelemetry struct {
	propagators    propagation.TextMapPropagator
	decoderSchema  *schema.Decoder
	recover        bool
	logStdOutPanic bool
}

func NewOtel(opts ...OptHttpOtelFunc) *opentelemetry {
	s := schema.NewDecoder()
	s.SetAliasTag("json")
	o := &opentelemetry{
		decoderSchema: s,
	}

	for _, opt := range opts {
		opt(o)
	}

	return o
}

func (o *opentelemetry) Trace(next http.HandlerFunc, opts ...Option) http.HandlerFunc {

	return func(writer http.ResponseWriter, request *http.Request) {

		ctx := request.Context()
		if o.propagators != nil {
			ctx = o.propagators.Extract(ctx, propagation.HeaderCarrier(request.Header))
		}

		recorder := &ResponseWriter{
			ResponseWriter: writer,
			logParams:      true,
			logRespBody:    true,
			logReqBody:     true,
		}
		for _, opt := range opts {
			opt(recorder)
		}

		ctx, span := otelTracer.Start(ctx, request.Method+" "+request.URL.Path, trace.WithAttributes(
			attribute.String("http.url", request.URL.String()),
			semconv.ServerAddress(request.Host),
			semconv.URLFull(request.URL.String()),
			attribute.String("http.host", request.Host),
			attribute.String("http.client_ip", request.RemoteAddr),
			attribute.String("http.target", request.URL.Path),
			attribute.String("http.request.method", request.Method),
			attribute.String("http.request.user_agent", request.UserAgent()),
			attribute.Int64("http.request.content_length", request.ContentLength),
		))
		defer func() {
			if r := recover(); r != nil {
				o.recoverHandler(writer, request, span, r)
			} else {
				span.End()
			}
		}()

		for k, v := range request.Header {
			headerValue := strings.Join(v, ", ")
			span.SetAttributes(attribute.String("http.request.header."+convertHeaderName(k), headerValue))
		}

		if o.propagators != nil {
			o.propagators.Inject(ctx, propagation.HeaderCarrier(request.Header))
			ctx = context.WithValue(ctx, TraceParent, request.Header.Get(TraceParent))
		}

		if recorder.logParams {
			queryParamToSpan(request, span)
		}

		ctx = context.WithValue(ctx, "log_req_body", recorder.logReqBody)
		request = request.WithContext(ctx)
		next.ServeHTTP(recorder, request)

		span.SetAttributes(
			attribute.Int("http.response.status_code", recorder.status),
			attribute.String("http.response.size.format", formatSize(recorder.size)),
			attribute.Int("http.response.size.raw", recorder.size),
		)

		for k, v := range recorder.Header() {
			headerValue := strings.Join(v, ", ")
			span.SetAttributes(attribute.String("http.request.header."+convertHeaderName(k), headerValue))
		}

		if recorder.logRespBody {
			span.SetAttributes(
				attribute.String("http.response.body", recorder.buffer.String()),
			)
		}

		span.SetName(fmt.Sprintf("%d %s %s", recorder.status, request.Method, request.URL.Path))
	}
}

func (o *opentelemetry) BindBodyRequest(w http.ResponseWriter, r *http.Request, v interface{}) bool {
	ctx := r.Context()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		setAttr(ctx, semconv.ErrorTypeKey.String("read_error"))
		o.Err(w, r, http.StatusUnprocessableEntity, err)
		return false
	}
	defer func() {
		if errReqBody := r.Body.Close(); errReqBody != nil {
			setAttr(ctx, semconv.ErrorTypeKey.String("close_error"))
			setAttr(ctx, semconv.ExceptionStacktrace(StackTrace(err).Error()))
		}
	}()

	if logReqBody, ok := ctx.Value(logReqBodyKey).(bool); ok && logReqBody {
		setAttr(ctx, attribute.String("http.request.body.json", string(body)))
	}

	err = json.Unmarshal(body, v)
	if err != nil {
		setAttr(ctx, semconv.ErrorTypeKey.String("unmarshal_error"))
		o.Err(w, r, http.StatusUnprocessableEntity, StackTrace(err))
		return false
	}
	return true
}

func (o *opentelemetry) WriteJson(w http.ResponseWriter, r *http.Request, code int, v interface{}) {
	ctx := r.Context()

	respByte, err := json.Marshal(v)
	if err != nil {
		setAttr(ctx, semconv.ErrorTypeKey.String("marshal_error"))
		o.Err(w, r, http.StatusInternalServerError, StackTrace(err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(respByte)
}

func (o *opentelemetry) BindQueryParam(w http.ResponseWriter, r *http.Request, v interface{}) bool {
	ctx := r.Context()

	if err := ParseQueryParam(r); err != nil {
		setAttr(ctx, semconv.ErrorTypeKey.String("parse_query_param"))
		o.Err(w, r, http.StatusBadRequest, StackTrace(err))
		return false
	}

	if err := o.decoderSchema.Decode(v, r.Form); err != nil {
		setAttr(ctx, semconv.ErrorTypeKey.String("decoder_schema"))
		o.Err(w, r, http.StatusBadRequest, StackTrace(err))
		return false
	}

	r.Form = nil

	//err := h.validator.Struct(v)
	//if err != nil {
	//	Error(w, r, http.StatusBadRequest, err)
	//	return false
	//}
	return true
}

// Err handles error responses by writing a JSON error message to the response writer.
func (o *opentelemetry) Err(w http.ResponseWriter, r *http.Request, code int, err error, messages ...string) {
	ctx := r.Context()

	setAttr(ctx, semconv.ExceptionMessage(err.Error()))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)

	msg := getMsgError(messages, code)
	var errMsgByte []byte
	var jsonErr error
	var writeDefaultErrorResponse = func() {
		RecordErrorOtel(ctx, err)
		w.Write([]byte(`{"error": "internal server error"}`))
	}

	var validationErrors validator.ValidationErrors
	if errors.As(err, &validationErrors) {
		errMsg := Error400{
			Errors: make(map[string][]string),
		}

		for _, validationError := range validationErrors {
			fieldName := validationError.Field()
			errMsg.Errors[fieldName] = []string{
				validationError.Error(),
			}
		}

		errMsgByte, jsonErr = json.Marshal(errMsg)
		if jsonErr != nil {
			writeDefaultErrorResponse()
			return
		}
	} else {
		errMsg := BasicError{
			Message: msg,
		}
		errMsgByte, jsonErr = json.Marshal(errMsg)
		if jsonErr != nil {
			writeDefaultErrorResponse()
			return
		}
	}

	if code >= 500 {
		RecordErrorOtel(ctx, err)
	}
	w.Write(errMsgByte)
}

func (o *opentelemetry) recoverHandler(writer http.ResponseWriter, request *http.Request, span trace.Span, r any) {
	span.SetAttributes(
		semconv.ExceptionTypeKey.String(fmt.Sprintf("%T", r)),
		semconv.ExceptionMessageKey.String(fmt.Sprintf("%v", r)),
	)
	if r == http.ErrAbortHandler {
		// we don't recover http.ErrAbortHandler so the response
		// to the client is aborted, this should not be logged
		span.RecordError(fmt.Errorf("panic: %v", r))
		span.End()
		panic(r)
	}
	if !o.recover {
		span.RecordError(fmt.Errorf("panic: %v", r))
		span.End()
		panic(r)
	}
	if o.logStdOutPanic {
		logEntry := middleware.GetLogEntry(request)
		if logEntry != nil {
			logEntry.Panic(r, debug.Stack())
		} else {
			middleware.PrintPrettyStack(r)
		}
	}

	if request.Header.Get("Connection") != "Upgrade" {
		o.Err(writer, request, http.StatusInternalServerError, fmt.Errorf("panic: %v", r))
	} else {
		span.RecordError(fmt.Errorf("panic: %v", r))
	}
	span.End()
}
