# http-log-wrap
`http-log-wrap` is a Go library designed to wrap the `http.ResponseWriter` with enhanced logging capabilities. This library provides a middleware for logging HTTP request and response details, including custom query parameters, request bodies, and response bodies. It also supports OpenTelemetry for advanced observability and includes middleware for handling standard Bearer token JWT authentication.

## Features 
- **Request Logging**: Logs HTTP request details including query parameters and request body.
- **Response Logging**: Logs HTTP response details including response body.
- **Customizable**: Configure which aspects of the request and response to log.
- **OpenTelemetry Support**: Integrates with OpenTelemetry for tracing and metrics.

## Installation
To install `http-log-wrap`, use the following Go command:
```shell3
go get github.com/SyaibanAhmadRamadhan/http-log-wrap@v1.240903.1054
```

## Tag Versioning Example: `v1.231215.2307`
We use a time-based versioning (TBD) scheme for our releases. The format is as follows:
```txt
v1.yearMonthDate.HourMinute
```
- `year`: Last two digits of the current year (e.g., 23 for 2023).
- `month`: Two-digit month (e.g., 12 for December).
- `date`: Two-digit day of the month (e.g., 15).
- `HourMinute`: Time of release in 24-hour format, combined as HHMM (e.g., 2307 for 11:07 PM).



## Contact
For questions or support, please contact ibanrama29@gmail.com