package apiserver

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	v1alpha1 "github.com/dcm-project/koku-cost-provider/api/v1alpha1"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
)

func rfc7807RecoveryMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
			defer func() {
				if rec := recover(); rec != nil {
					if rec == http.ErrAbortHandler { //nolint:goerr113 // stdlib sentinel
						panic(http.ErrAbortHandler)
					}
					logger.Error("panic recovered", "panic", rec, "stack", string(debug.Stack()))
					if rw.wroteHeader {
						return
					}
					writeRFC7807(w, logger, http.StatusInternalServerError, v1alpha1.INTERNAL, "Internal Server Error", "an unexpected error occurred")
				}
			}()
			next.ServeHTTP(rw, r)
		})
	}
}

func requestTimeoutMiddleware(timeout time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func requestLoggingMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
			next.ServeHTTP(rw, r)
			logger.Info("request completed",
				"method", r.Method,
				"path", r.URL.Path,
				"status", rw.statusCode,
				"duration", time.Since(start).String(),
			)
		})
	}
}

func bodySizeLimitMiddleware(maxBytes int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil && maxBytes > 0 {
				r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			}
			next.ServeHTTP(w, r)
		})
	}
}

func openAPIValidationMiddleware(logger *slog.Logger, specRouter routers.Router, badReq func(http.ResponseWriter, *http.Request, error)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			route, pathParams, err := specRouter.FindRoute(r)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			var bodyBytes []byte
			if r.Body != nil {
				bodyBytes, err = io.ReadAll(r.Body)
				if err != nil {
					writeRFC7807(w, logger, http.StatusRequestEntityTooLarge, v1alpha1.INVALIDARGUMENT, "Request Entity Too Large", "request body too large")
					return
				}
				r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			}

			input := &openapi3filter.RequestValidationInput{
				Request:    r,
				PathParams: pathParams,
				Route:      route,
			}
			if err := openapi3filter.ValidateRequest(r.Context(), input); err != nil {
				detail := scrubValidationError(err)
				logger.Warn("request validation failed",
					"method", r.Method, "path", r.URL.Path, "detail", detail,
				)
				badReq(w, r, err)
				return
			}

			if bodyBytes != nil {
				r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			}
			next.ServeHTTP(w, r)
		})
	}
}
