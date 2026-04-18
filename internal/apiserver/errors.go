package apiserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	v1alpha1 "github.com/dcm-project/koku-cost-provider/api/v1alpha1"
	oapigen "github.com/dcm-project/koku-cost-provider/internal/api/server"
	"github.com/dcm-project/koku-cost-provider/internal/util"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
)

const contentTypeProblemJSON = "application/problem+json"

func writeRFC7807(w http.ResponseWriter, logger *slog.Logger, statusCode int, errType v1alpha1.ErrorType, title, detail string) {
	status := int32(statusCode)
	resp := v1alpha1.Error{
		Type:   errType,
		Title:  title,
		Status: util.Ptr(status),
		Detail: util.Ptr(detail),
	}
	w.Header().Set("Content-Type", contentTypeProblemJSON)
	w.WriteHeader(statusCode)
	if encErr := json.NewEncoder(w).Encode(resp); encErr != nil {
		logger.Error("failed to encode error response", "error", encErr)
	}
}

func newBadRequestHandler(logger *slog.Logger) func(http.ResponseWriter, *http.Request, error) {
	return func(w http.ResponseWriter, _ *http.Request, err error) {
		writeRFC7807(w, logger, http.StatusBadRequest, v1alpha1.INVALIDARGUMENT, "Bad Request", scrubValidationError(err))
	}
}

func scrubValidationError(err error) string {
	const genericMsg = "invalid request"

	var reqErr *openapi3filter.RequestError
	if errors.As(err, &reqErr) {
		var prefix string
		if p := reqErr.Parameter; p != nil {
			prefix = fmt.Sprintf("parameter %q in %s", p.Name, p.In)
		} else if reqErr.RequestBody != nil {
			prefix = "request body"
		}

		var schemaErr *openapi3.SchemaError
		if errors.As(reqErr.Err, &schemaErr) && schemaErr.Reason != "" {
			if prefix != "" {
				return prefix + ": " + schemaErr.Reason
			}
			return schemaErr.Reason
		}
		if reqErr.Reason != "" {
			if prefix != "" {
				return prefix + ": " + reqErr.Reason
			}
			return reqErr.Reason
		}
		return genericMsg
	}

	var paramErr *oapigen.InvalidParamFormatError
	if errors.As(err, &paramErr) {
		return fmt.Sprintf("invalid format for parameter %q", paramErr.ParamName)
	}
	var requiredErr *oapigen.RequiredParamError
	if errors.As(err, &requiredErr) {
		return fmt.Sprintf("missing required parameter %q", requiredErr.ParamName)
	}
	var unmarshalErr *oapigen.UnmarshalingParamError
	if errors.As(err, &unmarshalErr) {
		return fmt.Sprintf("invalid value for parameter %q", unmarshalErr.ParamName)
	}

	return genericMsg
}

type responseWriter struct {
	http.ResponseWriter
	statusCode  int
	wroteHeader bool
}

func (w *responseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.wroteHeader = true
	w.ResponseWriter.WriteHeader(code)
}

func (w *responseWriter) Write(b []byte) (int, error) {
	w.wroteHeader = true
	return w.ResponseWriter.Write(b)
}

func (w *responseWriter) Flush() {
	w.wroteHeader = true
	if fl, ok := w.ResponseWriter.(http.Flusher); ok {
		fl.Flush()
	}
}

func (w *responseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
