package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	taskdto "github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
)

func MidjourneyErrorWrapper(code int, desc string) *taskdto.MidjourneyResponse {
	return &taskdto.MidjourneyResponse{
		Code:        code,
		Description: desc,
	}
}

func MidjourneyErrorWithStatusCodeWrapper(code int, desc string, statusCode int) *taskdto.MidjourneyResponseWithStatusCode {
	return &taskdto.MidjourneyResponseWithStatusCode{
		StatusCode: statusCode,
		Response:   *MidjourneyErrorWrapper(code, desc),
	}
}

//// OpenAIErrorWrapper wraps an error into an OpenAIErrorWithStatusCode
//func OpenAIErrorWrapper(err error, code string, statusCode int) *dto.OpenAIErrorWithStatusCode {
//	text := err.Error()
//	lowerText := strings.ToLower(text)
//	if !strings.HasPrefix(lowerText, "get file base64 from url") && !strings.HasPrefix(lowerText, "mime type is not supported") {
//		if strings.Contains(lowerText, "post") || strings.Contains(lowerText, "dial") || strings.Contains(lowerText, "http") {
//			common.SysLog(fmt.Sprintf("error: %s", text))
//			text = "请求上游地址失败"
//		}
//	}
//	openAIError := dto.OpenAIError{
//		Message: text,
//		Type:    "new_api_error",
//		Code:    code,
//	}
//	return &dto.OpenAIErrorWithStatusCode{
//		Error:      openAIError,
//		StatusCode: statusCode,
//	}
//}
//
//func OpenAIErrorWrapperLocal(err error, code string, statusCode int) *dto.OpenAIErrorWithStatusCode {
//	openaiErr := OpenAIErrorWrapper(err, code, statusCode)
//	openaiErr.LocalError = true
//	return openaiErr
//}

func ClaudeErrorWrapper(err error, code string, statusCode int) *dto.ClaudeErrorWithStatusCode {
	text := err.Error()
	lowerText := strings.ToLower(text)
	if !strings.HasPrefix(lowerText, "get file base64 from url") {
		if strings.Contains(lowerText, "post") || strings.Contains(lowerText, "dial") || strings.Contains(lowerText, "http") {
			common.SysLog(fmt.Sprintf("error: %s", text))
			text = "请求上游地址失败"
		}
	}
	claudeError := types.ClaudeError{
		Message: text,
		Type:    "new_api_error",
	}
	return &dto.ClaudeErrorWithStatusCode{
		Error:      claudeError,
		StatusCode: statusCode,
	}
}

func ClaudeErrorWrapperLocal(err error, code string, statusCode int) *dto.ClaudeErrorWithStatusCode {
	claudeErr := ClaudeErrorWrapper(err, code, statusCode)
	claudeErr.LocalError = true
	return claudeErr
}

func RelayErrorHandler(ctx context.Context, resp *http.Response, showBodyWhenFail bool) (newApiErr *types.NewAPIError) {
	newApiErr = types.InitOpenAIError(types.ErrorCodeBadResponseStatusCode, resp.StatusCode)

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}
	CloseResponseBodyGracefully(resp)
	var errResponse dto.GeneralErrorResponse
	responseBodyText := string(responseBody)
	responseBodyPreview := common.LocalLogPreview(responseBodyText)
	buildErrWithBody := func(message string) error {
		if message == "" {
			return fmt.Errorf("bad response status code %d, body: %s", resp.StatusCode, responseBodyText)
		}
		return fmt.Errorf("bad response status code %d, message: %s, body: %s", resp.StatusCode, message, responseBodyText)
	}

	err = common.Unmarshal(responseBody, &errResponse)
	if err != nil {
		if showBodyWhenFail {
			newApiErr.Err = buildErrWithBody("")
		} else {
			logger.LogError(ctx, fmt.Sprintf("bad response status code %d, body: %s", resp.StatusCode, responseBodyPreview))
			newApiErr.Err = fmt.Errorf("bad response status code %d", resp.StatusCode)
		}
		return
	}

	if common.GetJsonType(errResponse.Error) == "object" {
		// General format error (OpenAI, Anthropic, Gemini, etc.)
		oaiError := errResponse.TryToOpenAIError()
		if oaiError != nil {
			newApiErr = types.WithOpenAIError(*oaiError, resp.StatusCode)
			if showBodyWhenFail {
				newApiErr.Err = buildErrWithBody(newApiErr.Error())
			}
			return
		}
	}
	message := errResponse.ToMessage()
	if message == "" {
		// The body parsed as JSON but carried no usable error message; log the
		// raw body so the upstream failure remains diagnosable.
		logger.LogError(ctx, fmt.Sprintf("bad response status code %d with empty error message, body: %s", resp.StatusCode, responseBodyPreview))
	}
	newApiErr = types.NewOpenAIError(errors.New(message), types.ErrorCodeBadResponseStatusCode, resp.StatusCode)
	if showBodyWhenFail {
		newApiErr.Err = buildErrWithBody(newApiErr.Error())
	}
	return
}

func ResetStatusCode(newApiErr *types.NewAPIError, statusCodeMappingStr string) {
	if newApiErr == nil {
		return
	}
	if statusCodeMappingStr == "" || statusCodeMappingStr == "{}" {
		return
	}
	statusCodeMapping := make(map[string]any)
	err := common.Unmarshal([]byte(statusCodeMappingStr), &statusCodeMapping)
	if err != nil {
		return
	}
	if newApiErr.StatusCode == http.StatusOK {
		return
	}
	codeStr := strconv.Itoa(newApiErr.StatusCode)
	if value, ok := statusCodeMapping[codeStr]; ok {
		intCode, ok := parseStatusCodeMappingValue(value)
		if !ok {
			return
		}
		newApiErr.StatusCode = intCode
	}
}

func parseStatusCodeMappingValue(value any) (int, bool) {
	switch v := value.(type) {
	case string:
		if v == "" {
			return 0, false
		}
		statusCode, err := strconv.Atoi(v)
		if err != nil {
			return 0, false
		}
		return statusCode, true
	case float64:
		if v != math.Trunc(v) {
			return 0, false
		}
		return int(v), true
	case int:
		return v, true
	case json.Number:
		statusCode, err := strconv.Atoi(v.String())
		if err != nil {
			return 0, false
		}
		return statusCode, true
	default:
		return 0, false
	}
}

func TaskErrorWrapperLocal(err error, code string, statusCode int) *taskdto.TaskError {
	openaiErr := TaskErrorWrapper(err, code, statusCode)
	openaiErr.LocalError = true
	return openaiErr
}

func TaskErrorWrapper(err error, code string, statusCode int) *taskdto.TaskError {
	text := err.Error()
	lowerText := strings.ToLower(text)
	if strings.Contains(lowerText, "post") || strings.Contains(lowerText, "dial") || strings.Contains(lowerText, "http") {
		common.SysLog(fmt.Sprintf("error: %s", text))
		//text = "请求上游地址失败"
		text = common.MaskSensitiveInfo(text)
	}
	//避免暴露内部错误
	taskError := &taskdto.TaskError{
		Code:       code,
		Message:    text,
		StatusCode: statusCode,
		Error:      err,
	}

	return taskError
}

// TaskErrorFromAPIError 将 PreConsumeBilling 返回的 NewAPIError 转换为 TaskError。
func TaskErrorFromAPIError(apiErr *types.NewAPIError) *taskdto.TaskError {
	if apiErr == nil {
		return nil
	}
	return &taskdto.TaskError{
		Code:       string(apiErr.GetErrorCode()),
		Message:    apiErr.Err.Error(),
		StatusCode: apiErr.StatusCode,
		Error:      apiErr.Err,
	}
}

// ErrorOverride is the channel-level configuration that rewrites the error
// response sent back to the client. Rules are evaluated in order and the
// first matching rule wins. Unlike StatusCodeMapping, which mutates the
// status code BEFORE relay retry/disable decisions are made, ErrorOverride is
// applied at the moment the response is about to be written to the client —
// the gateway's internal retry / disable / billing logic still sees the
// original upstream status.
//
// The wire-format JSON for this configuration is a top-level array of rules:
//
//	[
//	  { "match": {...}, "override": {...} },
//	  ...
//	]
type ErrorOverrideRule struct {
	Match    ErrorOverrideMatch    `json:"match"`
	Override ErrorOverrideOverride `json:"override"`
}

// ErrorOverrideMatch narrows when the rule fires. Every field is optional;
// fields left empty are treated as "always match". A rule with all fields
// empty matches every error.
type ErrorOverrideMatch struct {
	HTTPStatus *int   `json:"http_status,omitempty"`
	Code       string `json:"code,omitempty"`
	Type       string `json:"type,omitempty"`
}

// ErrorOverrideOverride describes the new values for the wire fields. Any
// field left empty / nil is left alone.
type ErrorOverrideOverride struct {
	HTTPStatus *int                  `json:"http_status,omitempty"`
	Body       ErrorOverrideBodySpec `json:"body,omitempty"`
}

type ErrorOverrideBodySpec struct {
	Error ErrorOverrideErrorSpec `json:"error,omitempty"`
}

type ErrorOverrideErrorSpec struct {
	Message string `json:"message,omitempty"`
	Type    string `json:"type,omitempty"`
	Code    string `json:"code,omitempty"`
	Param   string `json:"param,omitempty"`
}

// ApplyErrorOverride inspects err against rulesStr (the channel's
// ErrorOverride JSON) and, on the first matching rule, mutates err to carry
// the override values. It is safe to call with an empty rulesStr, a nil err,
// or malformed JSON — in all of these cases the function is a no-op.
func ApplyErrorOverride(err *types.NewAPIError, rulesStr string) {
	if err == nil {
		return
	}
	if rulesStr == "" || rulesStr == "{}" || rulesStr == "[]" {
		return
	}
	var rules []ErrorOverrideRule
	if uerr := common.Unmarshal([]byte(rulesStr), &rules); uerr != nil {
		return
	}
	if len(rules) == 0 {
		return
	}
	for i := range rules {
		if !errorOverrideRuleMatches(err, &rules[i].Match) {
			continue
		}
		applyErrorOverride(err, &rules[i].Override)
		return
	}
}

func errorOverrideRuleMatches(err *types.NewAPIError, m *ErrorOverrideMatch) bool {
	if m == nil {
		return true
	}
	if m.HTTPStatus != nil && err.StatusCode != *m.HTTPStatus {
		return false
	}
	if m.Code != "" {
		if wireErrorCode(err) != m.Code {
			return false
		}
	}
	if m.Type != "" {
		if wireErrorType(err) != m.Type {
			return false
		}
	}
	return true
}

func applyErrorOverride(err *types.NewAPIError, o *ErrorOverrideOverride) {
	if o == nil {
		return
	}
	if o.HTTPStatus != nil {
		err.StatusCode = *o.HTTPStatus
	}
	spec := o.Body.Error
	patch := types.WireErrorPatch{}
	if spec.Message != "" {
		patch.Message = &spec.Message
	}
	if spec.Type != "" {
		patch.Type = &spec.Type
	}
	if spec.Code != "" {
		patch.Code = &spec.Code
	}
	if spec.Param != "" {
		patch.Param = &spec.Param
	}
	if patch.Message != nil || patch.Type != nil || patch.Code != nil || patch.Param != nil {
		err.OverrideWireFields(patch)
	}
}

// wireErrorType extracts the user-visible "type" field of the underlying
// wire-format error. Falls back to the NewAPIError's internal error type
// when the payload does not carry one.
func wireErrorType(err *types.NewAPIError) string {
	switch relayErr := err.RelayError.(type) {
	case types.OpenAIError:
		return relayErr.Type
	case types.ClaudeError:
		return relayErr.Type
	}
	return string(err.GetErrorType())
}

// wireErrorCode extracts the user-visible "code" field of the underlying
// wire-format error. Claude errors have no code; we return "" there.
func wireErrorCode(err *types.NewAPIError) string {
	switch relayErr := err.RelayError.(type) {
	case types.OpenAIError:
		switch c := relayErr.Code.(type) {
		case string:
			return c
		case nil:
			return ""
		default:
			return fmt.Sprintf("%v", c)
		}
	}
	return ""
}
