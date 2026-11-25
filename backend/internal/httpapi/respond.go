package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/chandug3121/mapup_geo-fence/backend/internal/domain"
)

// apiFunc is the shape of every JSON endpoint: it returns a status code and
// a body (a struct or map that encodes to a JSON object), or an error.
type apiFunc func(r *http.Request) (status int, body any, err error)

// handle turns an apiFunc into an http.HandlerFunc. It measures how long the
// handler took with the time package and adds it to the response as
// "time_ns" (a string, as the spec requires). Because this happens in one
// place, no endpoint — including error responses — can forget it.
func (a *API) handle(fn apiFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		status, body, err := fn(r)
		if err != nil {
			status, body = a.errorResponse(r, err)
		}
		writeJSON(w, status, body, time.Since(start))
	}
}

// errorBody is the JSON shape of every error response.
type errorBody struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func newError(code, message string) errorBody {
	return errorBody{Error: errorDetail{Code: code, Message: message}}
}

// badRequestError is returned for malformed requests (bad JSON, bad query
// parameters) that aren't tied to a domain rule.
type badRequestError struct{ msg string }

func (e *badRequestError) Error() string { return e.msg }

func badRequest(format string, args ...any) error {
	return &badRequestError{msg: fmt.Sprintf(format, args...)}
}

// errorResponse maps an error to a status code and a user-friendly body.
// Unexpected errors are logged and hidden behind a generic message.
func (a *API) errorResponse(r *http.Request, err error) (int, errorBody) {
	var (
		validation *domain.ValidationError
		badReq     *badRequestError
	)
	switch {
	case errors.As(err, &validation):
		return http.StatusBadRequest, newError("validation_error", validation.Error())
	case errors.As(err, &badReq):
		return http.StatusBadRequest, newError("bad_request", badReq.msg)
	case errors.Is(err, domain.ErrNotFound):
		return http.StatusNotFound, newError("not_found", err.Error())
	case errors.Is(err, domain.ErrConflict):
		return http.StatusConflict, newError("conflict", strings.TrimSuffix(err.Error(), ": "+domain.ErrConflict.Error()))
	default:
		a.log.Error("request failed", "method", r.Method, "path", r.URL.Path, "error", err)
		return http.StatusInternalServerError, newError("internal_error", "Something went wrong on our side. Please try again.")
	}
}

// writeJSON encodes body (which must encode to a JSON object) and appends
// "time_ns" as the last field.
func writeJSON(w http.ResponseWriter, status int, body any, elapsed time.Duration) {
	payload, err := json.Marshal(body)
	if err != nil || len(payload) < 2 || payload[0] != '{' {
		status = http.StatusInternalServerError
		payload = []byte(`{"error":{"code":"internal_error","message":"failed to encode response"}}`)
	}

	var buf bytes.Buffer
	buf.Grow(len(payload) + 32)
	buf.Write(payload[:len(payload)-1]) // everything except the closing brace
	if len(payload) > 2 {               // not an empty object
		buf.WriteByte(',')
	}
	buf.WriteString(`"time_ns":"`)
	buf.WriteString(strconv.FormatInt(elapsed.Nanoseconds(), 10))
	buf.WriteString(`"}`)
	buf.WriteByte('\n')

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(buf.Bytes())
}

// decodeJSON reads a single JSON object from the request body into dst and
// turns decoder errors into messages a person can act on.
func decodeJSON(r *http.Request, dst any) error {
	if r.Body == nil {
		return badRequest("request body is required")
	}
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(dst); err != nil {
		var (
			syntaxErr *json.SyntaxError
			typeErr   *json.UnmarshalTypeError
			maxErr    *http.MaxBytesError
		)
		switch {
		case errors.Is(err, io.EOF):
			return badRequest("request body is required")
		case errors.As(err, &syntaxErr):
			return badRequest("request body is not valid JSON (error at position %d)", syntaxErr.Offset)
		case errors.Is(err, io.ErrUnexpectedEOF):
			return badRequest("request body is not valid JSON (it ends too early)")
		case errors.As(err, &typeErr):
			if typeErr.Field != "" {
				return badRequest("field %q has the wrong type (expected %s)", typeErr.Field, typeErr.Type)
			}
			return badRequest("request body must be a JSON object")
		case errors.As(err, &maxErr):
			return badRequest("request body is too large (max %d bytes)", maxErr.Limit)
		default:
			return badRequest("could not read request body: %v", err)
		}
	}
	if dec.More() {
		return badRequest("request body must contain a single JSON object")
	}
	return nil
}
