package gateway

import (
	"net/http"
	"testing"

	"google.golang.org/grpc/codes"
)

func TestHTTPStatusMapping(t *testing.T) {
	cases := map[codes.Code]int{
		codes.InvalidArgument:    http.StatusBadRequest,
		codes.NotFound:           http.StatusNotFound,
		codes.AlreadyExists:      http.StatusConflict,
		codes.FailedPrecondition: http.StatusConflict,
		codes.Unavailable:        http.StatusServiceUnavailable,
		codes.DeadlineExceeded:   http.StatusGatewayTimeout,
		codes.Internal:           http.StatusInternalServerError,
		codes.Unknown:            http.StatusInternalServerError,
	}
	for code, want := range cases {
		if got := HTTPStatus(code); got != want {
			t.Errorf("HTTPStatus(%s) = %d, want %d", code, got, want)
		}
	}
}
