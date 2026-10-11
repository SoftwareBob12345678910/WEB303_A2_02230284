package gateway

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// protoJSON renders protobuf messages with snake_case names (matching the
// .proto files) and includes zero values so clients always see every field.
var protoJSON = protojson.MarshalOptions{UseProtoNames: true, EmitUnpopulated: true}

type apiError struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

// HTTPStatus maps a gRPC status code to an HTTP status code.
// This table is the heart of "mapping gRPC errors to HTTP" (Requirement 4b).
func HTTPStatus(c codes.Code) int {
	switch c {
	case codes.OK:
		return http.StatusOK
	case codes.InvalidArgument:
		return http.StatusBadRequest // 400
	case codes.NotFound:
		return http.StatusNotFound // 404
	case codes.AlreadyExists, codes.FailedPrecondition:
		return http.StatusConflict // 409
	case codes.Unauthenticated:
		return http.StatusUnauthorized // 401
	case codes.PermissionDenied:
		return http.StatusForbidden // 403
	case codes.Unavailable:
		return http.StatusServiceUnavailable // 503
	case codes.DeadlineExceeded:
		return http.StatusGatewayTimeout // 504
	default:
		return http.StatusInternalServerError // 500
	}
}

// writeError converts any error from a gRPC call into a JSON HTTP response.
func writeError(w http.ResponseWriter, err error) {
	st, _ := status.FromError(err) // non-gRPC errors become codes.Unknown
	httpCode := HTTPStatus(st.Code())

	msg := st.Message()
	if httpCode == http.StatusInternalServerError {
		log.Printf("internal error from backend: %v", err)
		msg = "internal server error" // never leak internals to clients
	}
	if httpCode == http.StatusServiceUnavailable {
		w.Header().Set("Retry-After", "10")
	}
	writeValue(w, httpCode, apiError{Error: msg, Code: st.Code().String()})
}

// writeAPIError writes an error that originates in the gateway itself.
func writeAPIError(w http.ResponseWriter, httpCode int, code codes.Code, msg string) {
	writeValue(w, httpCode, apiError{Error: msg, Code: code.String()})
}

func writeProto(w http.ResponseWriter, httpCode int, m proto.Message) {
	b, err := protoJSON.Marshal(m)
	if err != nil {
		log.Printf("marshal response: %v", err)
		writeAPIError(w, http.StatusInternalServerError, codes.Internal, "internal server error")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpCode)
	_, _ = w.Write(b)
}

func writeValue(w http.ResponseWriter, httpCode int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpCode)
	_ = json.NewEncoder(w).Encode(v)
}

// readJSON decodes the request body into a protobuf message. Unknown fields,
// wrong types and malformed JSON are rejected with 400 before any backend call.
// It returns false after writing the error response.
func readJSON(w http.ResponseWriter, r *http.Request, m proto.Message) bool {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20)) // 1 MiB cap
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, codes.InvalidArgument, "could not read request body")
		return false
	}
	if err := protojson.Unmarshal(body, m); err != nil {
		writeAPIError(w, http.StatusBadRequest, codes.InvalidArgument, "invalid JSON body: "+err.Error())
		return false
	}
	return true
}

// logRequests prints one access-log line per request.
func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		log.Printf("http %s %s -> %d (%s)", r.Method, r.URL.RequestURI(), rec.status, time.Since(start).Round(time.Millisecond))
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}
