# http-log-wrap
`http-log-wrap` is a Go library designed to wrap the `http.ResponseWriter` with enhanced logging capabilities. This library provides a middleware for logging HTTP request and response details, including custom query parameters, request bodies, and response bodies. It also supports OpenTelemetry for advanced observability and includes middleware for handling standard Bearer token JWT authentication.

## Features 
- **Request Logging**: Logs HTTP request details including query parameters and request body.
- **Response Logging**: Logs HTTP response details including response body.
- **Customizable**: Configure which aspects of the request and response to log.
- **OpenTelemetry Support**: Integrates with OpenTelemetry for tracing and metrics.
- **JWT Authentication**: Middleware for handling standard Bearer token JWT authentication.

## Installation
To install `http-log-wrap`, use the following Go command:
```shell
go get github.com/yourusername/http-log-wrap
```

## Contact
For questions or support, please contact ibanrama29@gmail.com