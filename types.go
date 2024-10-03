package whttp

type BasicError struct {
	Message string `json:"message"`
}

type Error400 struct {
	Errors map[string][]string `json:"errors"`
}

const TraceParent = "traceparent"
const logReqBodyKey = "log_req_body"
