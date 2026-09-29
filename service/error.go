package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"reflect"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	taskdto "github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"

	"github.com/gin-gonic/gin"
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
// The engine treats the outgoing error as a generic JSON tree:
//
//	{ "status_code": 503, "error": { "message": "...", "type": "...", "code": "...", "param": "..." } }
//
// match and override address that tree with dot paths, so any upstream field
// can be matched and any field can be rewritten; paths not mentioned in
// override are returned to the client unchanged. The wire-format JSON is a
// top-level array of rules:
//
//	[
//	  { "match": { "status_code": 503, "error.code": "x" },
//	    "override": { "error.message": "服务繁忙", "status_code": 429 } },
//	  ...
//	]
type ErrorOverrideRule struct {
	Match    map[string]any `json:"match"`
	Override map[string]any `json:"override"`
}

// errorOverrideTree renders err as the generic JSON tree that match/override
// paths address: the wire error under "error" and the status under
// "status_code".
func errorOverrideTree(err *types.NewAPIError) map[string]any {
	tree := map[string]any{"status_code": err.StatusCode}
	switch relayErr := err.RelayError.(type) {
	case types.OpenAIError:
		wire := map[string]any{
			"message": relayErr.Message,
			"type":    relayErr.Type,
			"param":   relayErr.Param,
		}
		switch code := relayErr.Code.(type) {
		case string:
			wire["code"] = code
		case nil:
		default:
			wire["code"] = fmt.Sprintf("%v", code)
		}
		tree["error"] = wire
	case types.ClaudeError:
		tree["error"] = map[string]any{
			"message": relayErr.Message,
			"type":    relayErr.Type,
		}
	default:
		tree["error"] = map[string]any{
			"message": err.Error(),
			"type":    string(err.GetErrorType()),
			"code":    string(err.GetErrorCode()),
		}
	}
	return tree
}

// errorOverridePath resolves a dot path like "error.message" inside tree,
// creating intermediate maps along the way when create is true. Parent
// segments must be objects; addressing into arrays or scalars returns nil.
func errorOverridePath(tree map[string]any, path string, create bool) any {
	parts := strings.Split(path, ".")
	if len(parts) == 0 || path == "" {
		return nil
	}
	current := tree
	for i, part := range parts {
		if part == "" {
			return nil
		}
		if i == len(parts)-1 {
			if !create {
				return current[part]
			}
			return current[part]
		}
		child, ok := current[part].(map[string]any)
		if !ok {
			if !create {
				return nil
			}
			if current[part] != nil {
				// Existing non-object value blocks descent.
				return nil
			}
			child = map[string]any{}
			current[part] = child
		}
		current = child
	}
	return nil
}

// errorOverrideRuleMatches reports whether every match path equals the value
// currently in the tree. A missing path only matches a null match value.
func errorOverrideRuleMatches(tree map[string]any, match map[string]any) bool {
	for path, want := range match {
		got := errorOverridePath(tree, path, false)
		if !reflect.DeepEqual(normalizeOverrideValue(got), normalizeOverrideValue(want)) {
			return false
		}
	}
	return true
}

// normalizeOverrideValue converts JSON-decoded numbers (float64) to int when
// whole so that status codes compare naturally across config shapes.
func normalizeOverrideValue(v any) any {
	switch value := v.(type) {
	case float64:
		if value == math.Trunc(value) && !math.IsInf(value, 0) {
			return int(value)
		}
	case json.Number:
		if i, err := value.Int64(); err == nil {
			return int(i)
		}
	}
	return v
}

// applyErrorOverrideTree writes every override path into the tree. A nil
// value deletes the target field, mirroring "not part of the response".
// Paths are dot-separated ("status_code", "error.message", ...).
func applyErrorOverrideTree(tree map[string]any, override map[string]any) {
	for path, value := range override {
		parts := strings.Split(path, ".")
		if len(parts) == 0 || path == "" {
			continue
		}
		parent := tree
		blocked := false
		for _, part := range parts[:len(parts)-1] {
			if part == "" {
				blocked = true
				break
			}
			child, ok := parent[part].(map[string]any)
			if !ok {
				if parent[part] != nil {
					blocked = true
					break
				}
				child = map[string]any{}
				parent[part] = child
			}
			parent = child
		}
		if blocked || parts[len(parts)-1] == "" {
			continue
		}
		key := parts[len(parts)-1]
		if value == nil {
			delete(parent, key)
		} else {
			parent[key] = value
		}
	}
}

// ApplyErrorOverride inspects err against rulesStr (the channel's
// ErrorOverride JSON) and, on the first matching rule, rewrites the wire
// fields of err in place. It is safe to call with an empty rulesStr, a nil
// err, or malformed JSON — in all of these cases the function is a no-op.
// Only upstream-originated errors are eligible; local gateway errors keep
// their messages.
func ApplyErrorOverride(err *types.NewAPIError, rulesStr string) {
	if err == nil {
		return
	}
	if err.GetErrorType() == types.ErrorTypeNewAPIError {
		return
	}
	rules, ok := parseErrorOverrideRules(rulesStr)
	if !ok {
		return
	}
	tree := errorOverrideTree(err)
	for i := range rules {
		if !errorOverrideRuleMatches(tree, rules[i].Match) {
			continue
		}
		if len(rules[i].Override) > 0 {
			applyErrorOverrideTree(tree, rules[i].Override)
			restoreErrorOverrideTree(err, tree)
		}
		return
	}
}

// parseErrorOverrideRules decodes the rules array. The bool is false when
// rulesStr holds no usable rules.
func parseErrorOverrideRules(rulesStr string) ([]ErrorOverrideRule, bool) {
	if rulesStr == "" || rulesStr == "{}" || rulesStr == "[]" {
		return nil, false
	}
	var rules []ErrorOverrideRule
	if uerr := common.Unmarshal([]byte(rulesStr), &rules); uerr != nil {
		return nil, false
	}
	for i := range rules {
		if rules[i].Match == nil {
			rules[i].Match = map[string]any{}
		}
		if rules[i].Override == nil {
			rules[i].Override = map[string]any{}
		}
	}
	return rules, len(rules) > 0
}

// restoreErrorOverrideTree writes the rewritten tree back into err's wire
// error, keeping the wire structs authoritative for serialization.
func restoreErrorOverrideTree(err *types.NewAPIError, tree map[string]any) {
	if status, ok := normalizeOverrideValue(tree["status_code"]).(int); ok && status > 0 && status < 600 {
		err.StatusCode = status
	}
	wire, _ := tree["error"].(map[string]any)
	if wire == nil {
		return
	}
	patch := types.WireErrorPatch{}
	setIf := func(field string, dst **string) {
		if raw, present := wire[field]; present {
			if raw == nil {
				empty := ""
				*dst = &empty
				return
			}
			if text, ok := raw.(string); ok {
				*dst = &text
			}
		}
	}
	setIf("message", &patch.Message)
	setIf("type", &patch.Type)
	setIf("code", &patch.Code)
	setIf("param", &patch.Param)
	if patch.Message != nil || patch.Type != nil || patch.Code != nil || patch.Param != nil {
		err.OverrideWireFields(patch)
	}
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

// ErrorOverrideSnapshot describes one side of an error override rewrite: the
// wire-format error as it arrived from upstream ("original") or as the client
// will receive it ("overridden"). The original message is masked like every
// other admin-facing error record.
type ErrorOverrideSnapshot struct {
	Message string `json:"message,omitempty"`
	Status  int    `json:"status,omitempty"`
	Type    string `json:"type,omitempty"`
	Code    string `json:"code,omitempty"`
	Param   string `json:"param,omitempty"`
}

// ErrorOverrideAudit records an error response override hit in the error log:
// the original upstream error and the rewritten error the client receives.
// It is evaluated when the log row is written, before the override is applied
// to the response; the rules and the error are unchanged in between, so the
// record and the client response always describe the same rewrite.
type ErrorOverrideAudit struct {
	Original   ErrorOverrideSnapshot `json:"original"`
	Overridden ErrorOverrideSnapshot `json:"overridden"`
}

// wireErrorSnapshot captures the user-visible wire fields of the error — the
// fields an override can rewrite and the ones the client would receive.
func wireErrorSnapshot(err *types.NewAPIError) ErrorOverrideSnapshot {
	snapshot := ErrorOverrideSnapshot{Status: err.StatusCode, Message: err.Error(), Type: string(err.GetErrorType())}
	switch relayErr := err.RelayError.(type) {
	case types.OpenAIError:
		snapshot.Message = relayErr.Message
		snapshot.Type = relayErr.Type
		snapshot.Param = relayErr.Param
		snapshot.Code = wireErrorCode(err)
	case types.ClaudeError:
		snapshot.Message = relayErr.Message
		snapshot.Type = relayErr.Type
	}
	return snapshot
}

// evaluateErrorOverride inspects err against rulesStr and returns the audit
// for the first matching rule without mutating err. It returns nil when no
// rule matches or the rules are absent or malformed.
func evaluateErrorOverride(err *types.NewAPIError, rulesStr string) *ErrorOverrideAudit {
	if err == nil || err.GetErrorType() == types.ErrorTypeNewAPIError {
		return nil
	}
	rules, ok := parseErrorOverrideRules(rulesStr)
	if !ok {
		return nil
	}
	tree := errorOverrideTree(err)
	for i := range rules {
		if !errorOverrideRuleMatches(tree, rules[i].Match) {
			continue
		}
		original := wireErrorSnapshot(err)
		original.Message = common.MaskSensitiveInfo(original.Message)
		overridden := original
		// Replay the rewrite on a copy of the full tree so paths like
		// "status_code" and "error.message" land exactly as they would for
		// the real response, then read the wire fields back.
		rewritten := map[string]any{"status_code": tree["status_code"], "error": map[string]any{}}
		for k, v := range tree["error"].(map[string]any) {
			rewritten["error"].(map[string]any)[k] = v
		}
		applyErrorOverrideTree(rewritten, rules[i].Override)
		wire, _ := rewritten["error"].(map[string]any)
		overridden.Message, _ = wire["message"].(string)
		overridden.Type, _ = wire["type"].(string)
		overridden.Code, _ = wire["code"].(string)
		overridden.Param, _ = wire["param"].(string)
		if status, ok := normalizeOverrideValue(rewritten["status_code"]).(int); ok {
			overridden.Status = status
		}
		// Claude wire errors carry neither code nor param, so OverrideWireFields
		// silently drops both there — the audit must not record them either.
		if _, ok := err.RelayError.(types.OpenAIError); !ok {
			overridden.Code, overridden.Param = "", ""
		}
		return &ErrorOverrideAudit{Original: original, Overridden: overridden}
	}
	return nil
}

// PrepareErrorOverrideAudit evaluates the channel error override rules against
// err and stores the audit for the error log written by ProcessChannelError.
// Each failed attempt replaces the stored audit, so a match from an earlier
// attempt can never reach a later attempt's log row. The response rewrite
// itself stays in ApplyErrorOverride, which re-evaluates the same unchanged
// error against the same rules.
func PrepareErrorOverrideAudit(c *gin.Context, err *types.NewAPIError) {
	if c == nil {
		return
	}
	common.SetContextKey(c, constant.ContextKeyErrorOverrideAudit, evaluateErrorOverride(err, common.GetContextKeyString(c, constant.ContextKeyChannelErrorOverride)))
}
