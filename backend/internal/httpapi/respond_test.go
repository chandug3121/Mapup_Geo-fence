package httpapi

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/chandug3121/mapup_geo-fence/backend/internal/domain"
)

func decodeBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not a JSON object: %v\n%s", err, rec.Body.String())
	}
	return body
}

func assertTimeNS(t *testing.T, body map[string]any) {
	t.Helper()
	raw, ok := body["time_ns"].(string)
	if !ok {
		t.Fatalf("time_ns missing or not a string: %#v", body["time_ns"])
	}
	if _, err := strconv.ParseInt(raw, 10, 64); err != nil {
		t.Fatalf("time_ns %q is not an integer string", raw)
	}
}

func TestWriteJSONAddsTimeNS(t *testing.T) {
	for _, body := range []any{
		map[string]any{"id": "geo_1"},
		map[string]any{},
		struct {
			Name string `json:"name"`
		}{"x"},
	} {
		rec := httptest.NewRecorder()
		writeJSON(rec, http.StatusCreated, body, 1234*time.Nanosecond)

		if rec.Code != http.StatusCreated {
			t.Errorf("status = %d", rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
			t.Errorf("content type = %q", ct)
		}
		got := decodeBody(t, rec)
		if got["time_ns"] != "1234" {
			t.Errorf("time_ns = %#v, want \"1234\"", got["time_ns"])
		}
	}
}

func TestHandleMapsErrorsAndKeepsTimeNS(t *testing.T) {
	a := &API{log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	cases := []struct {
		err      error
		wantCode int
		wantType string
	}{
		{domain.Invalid("name", "is required"), http.StatusBadRequest, "validation_error"},
		{badRequest("bad"), http.StatusBadRequest, "bad_request"},
		{domain.NotFound("vehicle", "veh_x"), http.StatusNotFound, "not_found"},
		{domain.ErrConflict, http.StatusConflict, "conflict"},
		{io.ErrClosedPipe, http.StatusInternalServerError, "internal_error"},
	}
	for _, c := range cases {
		h := a.handle(func(*http.Request) (int, any, error) { return 0, nil, c.err })
		rec := httptest.NewRecorder()
		h(rec, httptest.NewRequest(http.MethodGet, "/x", nil))

		if rec.Code != c.wantCode {
			t.Errorf("%v: status = %d, want %d", c.err, rec.Code, c.wantCode)
		}
		body := decodeBody(t, rec)
		assertTimeNS(t, body)
		detail, _ := body["error"].(map[string]any)
		if detail["code"] != c.wantType {
			t.Errorf("%v: code = %v, want %s", c.err, detail["code"], c.wantType)
		}
	}
}

func TestInternalErrorsAreNotLeaked(t *testing.T) {
	a := &API{log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	h := a.handle(func(*http.Request) (int, any, error) {
		return 0, nil, io.ErrUnexpectedEOF
	})
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
	if strings.Contains(rec.Body.String(), "unexpected EOF") {
		t.Errorf("internal error text leaked to client: %s", rec.Body.String())
	}
}

func TestDecodeJSONMessages(t *testing.T) {
	cases := map[string]string{
		"":                 "request body is required",
		"{":                "ends too early",
		"{]":               "not valid JSON",
		`{"name": 5}`:      `field "name" has the wrong type`,
		`{"name":"a"} {}`:  "single JSON object",
		`["not","object"]`: "JSON object",
	}
	for input, want := range cases {
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(input))
		var dst struct {
			Name string `json:"name"`
		}
		err := decodeJSON(r, &dst)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("decodeJSON(%q) error = %v, want it to contain %q", input, err, want)
		}
	}
}
