package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"

	"oral-history-release-desk/internal/casefile"
	"oral-history-release-desk/internal/release"
	"oral-history-release-desk/internal/store"
)

const maxRequestBody = 1 << 20

type errorEnvelope struct {
	Error apiError `json:"error"`
}

type apiError struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
	Details any               `json:"details,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	response := apiError{Code: "internal_error", Message: "服务处理请求时发生错误"}
	var input releaseInputError
	if errors.As(err, &input) {
		status = http.StatusBadRequest
		response = apiError{Code: "bad_request", Message: input.Error()}
	}
	if errors.Is(err, store.ErrNotFound) {
		status = http.StatusNotFound
		response = apiError{Code: "not_found", Message: "案件不存在"}
	}
	var validation casefile.ValidationError
	if errors.As(err, &validation) {
		status = http.StatusUnprocessableEntity
		response = apiError{Code: "validation_failed", Message: validation.Message, Fields: map[string]string{validation.Field: validation.Message}}
	}
	var multi casefile.MultiValidationError
	if errors.As(err, &multi) {
		status = http.StatusUnprocessableEntity
		response = apiError{Code: "validation_failed", Message: multi.Error(), Fields: multi.Fields}
	}
	var decisionConflict casefile.DecisionConflictError
	if errors.As(err, &decisionConflict) {
		status = http.StatusConflict
		response = apiError{Code: "decision_conflict", Message: decisionConflict.Error(), Details: decisionConflict}
	}
	var state casefile.StateError
	if errors.As(err, &state) {
		status = http.StatusConflict
		response = apiError{Code: "invalid_state", Message: state.Error()}
	}
	var version store.VersionConflictError
	if errors.As(err, &version) {
		status = http.StatusConflict
		response = apiError{Code: "version_conflict", Message: version.Error(), Fields: map[string]string{
			"expectedVersion": fmt.Sprint(version.Expected), "actualVersion": fmt.Sprint(version.Actual),
		}}
	}
	var idempotency store.IdempotencyConflictError
	if errors.As(err, &idempotency) {
		status = http.StatusConflict
		response = apiError{Code: "idempotency_conflict", Message: idempotency.Error()}
	}
	var integrity release.IntegrityError
	if errors.As(err, &integrity) {
		status = http.StatusConflict
		response = apiError{Code: "integrity_error", Message: integrity.Error(), Details: integrity}
	}
	if status == http.StatusInternalServerError && isClientError(err) {
		status = http.StatusBadRequest
		response = apiError{Code: "bad_request", Message: err.Error()}
	}
	writeJSON(w, status, errorEnvelope{Error: response})
}

func isClientError(err error) bool {
	message := err.Error()
	markers := []string{"不能为空", "必须", "缺少", "不存在", "尚未", "没有待", "仍有阻断", "已存在"}
	for _, marker := range markers {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}

func decodeJSON(w http.ResponseWriter, r *http.Request, destination any) error {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return releaseInputError("Content-Type 必须为 application/json")
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return releaseInputError("JSON 请求体无效: " + err.Error())
	}
	var extra any
	if err := decoder.Decode(&extra); err == nil {
		return fmt.Errorf("JSON 请求体只能包含一个对象")
	} else if err != io.EOF {
		return releaseInputError("JSON 请求体尾部无效: " + err.Error())
	}
	return nil
}

func caseID(r *http.Request) (string, error) {
	id := strings.TrimSpace(r.PathValue("caseID"))
	if id == "" || len(id) > 128 || strings.ContainsAny(id, "/\\") {
		return "", releaseInputError("案件 ID 格式无效")
	}
	return id, nil
}
