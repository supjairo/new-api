package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/types"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestResetStatusCode(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name             string
		statusCode       int
		statusCodeConfig string
		expectedCode     int
	}{
		{
			name:             "map string value",
			statusCode:       429,
			statusCodeConfig: `{"429":"503"}`,
			expectedCode:     503,
		},
		{
			name:             "map int value",
			statusCode:       429,
			statusCodeConfig: `{"429":503}`,
			expectedCode:     503,
		},
		{
			name:             "skip invalid string value",
			statusCode:       429,
			statusCodeConfig: `{"429":"bad-code"}`,
			expectedCode:     429,
		},
		{
			name:             "skip status code 200",
			statusCode:       200,
			statusCodeConfig: `{"200":503}`,
			expectedCode:     200,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			newAPIError := &types.NewAPIError{
				StatusCode: tc.statusCode,
			}
			ResetStatusCode(newAPIError, tc.statusCodeConfig)
			require.Equal(t, tc.expectedCode, newAPIError.StatusCode)
		})
	}
}

func TestRelayErrorHandlerTruncatesInvalidJSONBodyInLog(t *testing.T) {
	withDebugEnabled(t, false)

	body := strings.Repeat("b", common.LocalLogContentLimit+256)
	var logBuffer bytes.Buffer

	common.LogWriterMu.Lock()
	oldWriter := gin.DefaultErrorWriter
	gin.DefaultErrorWriter = &logBuffer
	common.LogWriterMu.Unlock()
	t.Cleanup(func() {
		common.LogWriterMu.Lock()
		gin.DefaultErrorWriter = oldWriter
		common.LogWriterMu.Unlock()
	})

	resp := &http.Response{
		StatusCode: http.StatusInternalServerError,
		Body:       io.NopCloser(strings.NewReader(body)),
	}

	newAPIError := RelayErrorHandler(context.Background(), resp, false)

	require.NotNil(t, newAPIError)
	require.Equal(t, "bad response status code 500", newAPIError.Error())
	require.Contains(t, logBuffer.String(), "[truncated")
	require.Contains(t, logBuffer.String(), fmt.Sprintf("original_length=%d", len(body)))
	require.NotContains(t, logBuffer.String(), strings.Repeat("b", common.LocalLogContentLimit+1))
}

func TestRelayErrorHandlerKeepsStructuredErrorMessage(t *testing.T) {
	message := strings.Repeat("c", common.LocalLogContentLimit+256)
	body := `{"message":"` + message + `"}`
	resp := &http.Response{
		StatusCode: http.StatusInternalServerError,
		Body:       io.NopCloser(strings.NewReader(body)),
	}

	newAPIError := RelayErrorHandler(context.Background(), resp, false)

	require.NotNil(t, newAPIError)
	require.Equal(t, message, newAPIError.Error())
}

func TestRelayErrorHandlerKeepsOpenAIErrorMessage(t *testing.T) {
	message := strings.Repeat("d", common.LocalLogContentLimit+256)
	body := `{"error":{"message":"` + message + `","type":"server_error","code":"server_error"}}`
	resp := &http.Response{
		StatusCode: http.StatusInternalServerError,
		Body:       io.NopCloser(strings.NewReader(body)),
	}

	newAPIError := RelayErrorHandler(context.Background(), resp, false)

	require.NotNil(t, newAPIError)
	require.Equal(t, message, newAPIError.Error())
}

func TestRelayErrorHandlerKeepsInvalidJSONBodyInDebugLog(t *testing.T) {
	withDebugEnabled(t, true)

	body := strings.Repeat("e", common.LocalLogContentLimit+256)
	var logBuffer bytes.Buffer

	common.LogWriterMu.Lock()
	oldWriter := gin.DefaultErrorWriter
	gin.DefaultErrorWriter = &logBuffer
	common.LogWriterMu.Unlock()
	t.Cleanup(func() {
		common.LogWriterMu.Lock()
		gin.DefaultErrorWriter = oldWriter
		common.LogWriterMu.Unlock()
	})

	resp := &http.Response{
		StatusCode: http.StatusInternalServerError,
		Body:       io.NopCloser(strings.NewReader(body)),
	}

	newAPIError := RelayErrorHandler(context.Background(), resp, false)

	require.NotNil(t, newAPIError)
	require.NotContains(t, logBuffer.String(), "[truncated")
	require.Contains(t, logBuffer.String(), body)
}

func withDebugEnabled(t *testing.T, enabled bool) {
	t.Helper()

	oldDebug := common.DebugEnabled
	common.DebugEnabled = enabled
	t.Cleanup(func() {
		common.DebugEnabled = oldDebug
	})
}

func TestApplyErrorOverride(t *testing.T) {
	t.Parallel()

	openaiErr := func(status int, code, typ, message string) *types.NewAPIError {
		return types.WithOpenAIError(types.OpenAIError{
			Message: message,
			Type:    typ,
			Code:    code,
		}, status)
	}

	t.Run("nil receiver is a no-op", func(t *testing.T) {
		t.Parallel()

		ApplyErrorOverride(nil, `[{"match":{}}]`)
	})

	t.Run("empty config leaves error untouched", func(t *testing.T) {
		t.Parallel()

		err := openaiErr(http.StatusServiceUnavailable, "smart_route_no_active_candidates", "new_api_error", "smart router empty")
		before := err.ToOpenAIError()
		ApplyErrorOverride(err, "")
		ApplyErrorOverride(err, "[]")
		ApplyErrorOverride(err, "not json")
		after := err.ToOpenAIError()

		require.Equal(t, before, after)
		require.Equal(t, http.StatusServiceUnavailable, err.StatusCode)
	})

	t.Run("match all empty matches any error", func(t *testing.T) {
		t.Parallel()

		err := openaiErr(http.StatusBadGateway, "whatever", "upstream_error", "boom")
		rules := `[{"match":{},"override":{"http_status":200,"body":{"error":{"message":"rewritten","type":"rewritten_type","code":"rewritten_code","param":"p"}}}}]`

		ApplyErrorOverride(err, rules)

		require.Equal(t, http.StatusOK, err.StatusCode)
		got := err.ToOpenAIError()
		require.Equal(t, "rewritten", got.Message)
		require.Equal(t, "rewritten_type", got.Type)
		require.Equal(t, "rewritten_code", got.Code)
		require.Equal(t, "p", got.Param)
	})

	t.Run("http_status must match", func(t *testing.T) {
		t.Parallel()

		err := openaiErr(http.StatusServiceUnavailable, "x", "y", "boom")
		rules := `[{"match":{"http_status":500},"override":{"body":{"error":{"message":"should not fire"}}}}]`

		ApplyErrorOverride(err, rules)

		require.Equal(t, "boom", err.ToOpenAIError().Message)
	})

	t.Run("code must match", func(t *testing.T) {
		t.Parallel()

		err := openaiErr(http.StatusServiceUnavailable, "smart_route_no_active_candidates", "y", "boom")
		rules := `[{"match":{"code":"other_code"},"override":{"body":{"error":{"message":"should not fire"}}}}]`

		ApplyErrorOverride(err, rules)

		require.Equal(t, "boom", err.ToOpenAIError().Message)
	})

	t.Run("type must match", func(t *testing.T) {
		t.Parallel()

		err := openaiErr(http.StatusServiceUnavailable, "x", "new_api_error", "boom")
		rules := `[{"match":{"type":"invalid_request_error"},"override":{"body":{"error":{"message":"should not fire"}}}}]`

		ApplyErrorOverride(err, rules)

		require.Equal(t, "boom", err.ToOpenAIError().Message)
	})

	t.Run("all three match conditions must hit", func(t *testing.T) {
		t.Parallel()

		err := openaiErr(http.StatusServiceUnavailable, "smart_route_no_active_candidates", "new_api_error", "boom")
		rules := `[{"match":{"http_status":503,"code":"smart_route_no_active_candidates","type":"new_api_error"},"override":{"http_status":200,"body":{"error":{"message":"pool busy"}}}}]`

		ApplyErrorOverride(err, rules)

		require.Equal(t, http.StatusOK, err.StatusCode)
		require.Equal(t, "pool busy", err.ToOpenAIError().Message)
	})

	t.Run("first match wins", func(t *testing.T) {
		t.Parallel()

		err := openaiErr(http.StatusServiceUnavailable, "c", "t", "boom")
		rules := `[
			{"match":{"http_status":503},"override":{"body":{"error":{"message":"first"}}}},
			{"match":{"http_status":503},"override":{"body":{"error":{"message":"second"}}}}
		]`

		ApplyErrorOverride(err, rules)

		require.Equal(t, "first", err.ToOpenAIError().Message)
	})

	t.Run("fallback rule matches after specific rule misses", func(t *testing.T) {
		t.Parallel()

		err := openaiErr(http.StatusServiceUnavailable, "different_code", "t", "boom")
		rules := `[
			{"match":{"code":"smart_route_no_active_candidates"},"override":{"body":{"error":{"message":"specific"}}}},
			{"match":{"http_status":503},"override":{"body":{"error":{"message":"fallback"}}}}
		]`

		ApplyErrorOverride(err, rules)

		require.Equal(t, "fallback", err.ToOpenAIError().Message)
	})

	t.Run("empty override fields keep original values", func(t *testing.T) {
		t.Parallel()

		err := openaiErr(http.StatusServiceUnavailable, "orig_code", "orig_type", "orig_message")
		rules := `[{"match":{"http_status":503},"override":{"body":{"error":{"message":"only message rewritten"}}}}]`

		ApplyErrorOverride(err, rules)

		got := err.ToOpenAIError()
		require.Equal(t, "only message rewritten", got.Message)
		require.Equal(t, "orig_type", got.Type)
		require.Equal(t, "orig_code", got.Code)
		require.Equal(t, http.StatusServiceUnavailable, err.StatusCode)
	})

	t.Run("message-only override keeps status code", func(t *testing.T) {
		t.Parallel()

		err := openaiErr(http.StatusServiceUnavailable, "c", "t", "original")
		rules := `[{"match":{"http_status":503},"override":{"body":{"error":{"message":"rewritten"}}}}]`

		ApplyErrorOverride(err, rules)

		require.Equal(t, http.StatusServiceUnavailable, err.StatusCode)
		require.Equal(t, "rewritten", err.ToOpenAIError().Message)
	})

	t.Run("claude wire format ignores code and param", func(t *testing.T) {
		t.Parallel()

		err := types.WithClaudeError(types.ClaudeError{
			Message: "claude boom",
			Type:    "upstream_error",
		}, http.StatusServiceUnavailable)
		rules := `[{"match":{"http_status":503},"override":{"body":{"error":{"message":"claude rewritten","type":"rewritten_type","code":"should_be_ignored","param":"ignored"}}}}]`

		ApplyErrorOverride(err, rules)

		got := err.ToClaudeError()
		require.Equal(t, "claude rewritten", got.Message)
		require.Equal(t, "rewritten_type", got.Type)
	})

	t.Run("claude matching uses claude type field", func(t *testing.T) {
		t.Parallel()

		err := types.WithClaudeError(types.ClaudeError{
			Message: "claude boom",
			Type:    "permission_error",
		}, http.StatusUnauthorized)
		rules := `[{"match":{"type":"permission_error"},"override":{"body":{"error":{"message":"matched"}}}}]`

		ApplyErrorOverride(err, rules)

		require.Equal(t, "matched", err.ToClaudeError().Message)
	})

	t.Run("override http_status of zero is valid", func(t *testing.T) {
		t.Parallel()

		err := openaiErr(http.StatusServiceUnavailable, "c", "t", "boom")
		rules := `[{"match":{"http_status":503},"override":{"http_status":200}}]`

		ApplyErrorOverride(err, rules)

		require.Equal(t, http.StatusOK, err.StatusCode)
	})

	t.Run("match with explicit zero status never matches a real error", func(t *testing.T) {
		t.Parallel()

		err := openaiErr(http.StatusServiceUnavailable, "c", "t", "boom")
		// Marshal a rule with http_status=0 by hand to verify that the pointer
		// is honoured when explicitly provided.
		rules := `[{"match":{"http_status":0},"override":{"body":{"error":{"message":"should not fire"}}}}]`

		ApplyErrorOverride(err, rules)

		require.Equal(t, "boom", err.ToOpenAIError().Message)
	})
}

func TestEvaluateErrorOverride(t *testing.T) {
	t.Parallel()

	t.Run("no rules or no match yields no audit", func(t *testing.T) {
		t.Parallel()

		err := types.WithOpenAIError(types.OpenAIError{Message: "boom", Type: "y", Code: "x"}, http.StatusServiceUnavailable)
		require.Nil(t, evaluateErrorOverride(err, ""))
		require.Nil(t, evaluateErrorOverride(err, "not json"))
		require.Nil(t, evaluateErrorOverride(err, `[{"match":{"http_status":500},"override":{"body":{"error":{"message":"m"}}}}]`))
	})

	t.Run("audit keeps the masked original and the overridden result", func(t *testing.T) {
		t.Parallel()

		err := types.WithOpenAIError(types.OpenAIError{
			Message: "upstream https://private.example.com/v1?token=secret-tok rejected: bad key",
			Type:    "invalid_request_error",
			Code:    "invalid_api_key",
		}, http.StatusUnauthorized)
		rules := `[{"match":{},"override":{"http_status":429,"body":{"error":{"message":"当前分组上游负载已饱和，请稍后再试","type":"rate_limit_error","code":"rate_limited"}}}}]`

		audit := evaluateErrorOverride(err, rules)

		require.NotNil(t, audit)
		require.Equal(t, http.StatusUnauthorized, audit.Original.Status)
		assert.NotContains(t, audit.Original.Message, "secret-tok")
		assert.NotContains(t, audit.Original.Message, "private.example.com")
		assert.Contains(t, audit.Original.Message, "rejected: bad key")
		assert.Equal(t, "invalid_api_key", audit.Original.Code)
		assert.Equal(t, "invalid_request_error", audit.Original.Type)
		require.Equal(t, http.StatusTooManyRequests, audit.Overridden.Status)
		assert.Equal(t, "当前分组上游负载已饱和，请稍后再试", audit.Overridden.Message)
		assert.Equal(t, "rate_limit_error", audit.Overridden.Type)
		assert.Equal(t, "rate_limited", audit.Overridden.Code)
		// Evaluation is read-only: the error stays untouched for the retry loop.
		require.Equal(t, http.StatusUnauthorized, err.StatusCode)
		require.Equal(t, "invalid_api_key", fmt.Sprintf("%v", err.ToOpenAIError().Code))
	})

	t.Run("claude errors carry no code or param", func(t *testing.T) {
		t.Parallel()

		err := types.WithClaudeError(types.ClaudeError{Type: "invalid_request_error", Message: "bad body"}, http.StatusBadRequest)
		rules := `[{"match":{},"override":{"body":{"error":{"message":"请求参数有误","code":"rewritten_code","param":"p"}}}}]`

		audit := evaluateErrorOverride(err, rules)

		require.NotNil(t, audit)
		assert.Equal(t, "bad body", audit.Original.Message)
		assert.Empty(t, audit.Original.Code)
		assert.Equal(t, "请求参数有误", audit.Overridden.Message)
		// OverrideWireFields drops code/param for Claude, so the audit does not record them either.
		assert.Empty(t, audit.Overridden.Code)
		assert.Empty(t, audit.Overridden.Param)
	})
}

func TestErrorOverrideAuditReachesErrorLog(t *testing.T) {
	previousDB, previousType := model.DB, common.MainDatabaseType()
	previousLogDB := model.LOG_DB
	previousCache, previousRedis := common.MemoryCacheEnabled, common.RedisEnabled
	previousErrorLog, previousAutoDisable := constant.ErrorLogEnabled, common.AutomaticDisableChannelEnabled
	t.Cleanup(func() {
		model.DB = previousDB
		model.LOG_DB = previousLogDB
		common.SetMainDatabaseType(previousType)
		common.MemoryCacheEnabled, common.RedisEnabled = previousCache, previousRedis
		constant.ErrorLogEnabled = previousErrorLog
		common.AutomaticDisableChannelEnabled = previousAutoDisable
	})
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := database.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, database.AutoMigrate(&model.Channel{}, &model.Ability{}, &model.User{}, &model.Log{}))
	model.DB = database
	model.LOG_DB = database
	common.SetMainDatabaseType(common.DatabaseTypeSQLite)
	common.MemoryCacheEnabled, common.RedisEnabled = false, false
	common.AutomaticDisableChannelEnabled = false
	constant.ErrorLogEnabled = true
	user := &model.User{Username: "override-audit", Role: common.RoleCommonUser, Status: common.UserStatusEnabled}
	require.NoError(t, database.Create(user).Error)

	rules := `[{"match":{"http_status":401},"override":{"http_status":429,"body":{"error":{"message":"服务繁忙，请稍后再试"}}}}]`
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	c.Set("id", user.Id)
	c.Set("token_name", "tok")
	c.Set("original_model", "gpt-test")
	c.Set("token_id", 1)
	c.Set("group", "default")
	common.SetContextKey(c, constant.ContextKeyChannelErrorOverride, rules)

	// First attempt fails with 401: the rule matches, the audit reaches the log.
	firstErr := types.WithOpenAIError(types.OpenAIError{Message: "invalid credential", Type: "invalid_request_error", Code: "invalid_api_key"}, http.StatusUnauthorized)
	PrepareErrorOverrideAudit(c, firstErr)
	ProcessChannelError(c, *types.NewChannelError(7, 1, "override-audit-ch", false, "fixture-key", false), firstErr, nil)

	// Second attempt fails with 500: the rule does not match, and the stale
	// audit from the first attempt must not leak into this log row.
	secondErr := types.WithOpenAIError(types.OpenAIError{Message: "internal boom", Type: "server_error", Code: "internal_error"}, http.StatusInternalServerError)
	PrepareErrorOverrideAudit(c, secondErr)
	ProcessChannelError(c, *types.NewChannelError(8, 1, "override-audit-ch", false, "fixture-key", false), secondErr, nil)

	var logs []model.Log
	require.NoError(t, database.Where("type = ?", model.LogTypeError).Order("id asc").Find(&logs).Error)
	require.Len(t, logs, 2)

	readAudit := func(other string) *ErrorOverrideAudit {
		var payload struct {
			AdminInfo struct {
				ErrorOverride *ErrorOverrideAudit `json:"error_override"`
			} `json:"admin_info"`
		}
		require.NoError(t, common.Unmarshal([]byte(other), &payload))
		return payload.AdminInfo.ErrorOverride
	}

	first := readAudit(logs[0].Other)
	require.NotNil(t, first, "a matched override records its audit under admin_info")
	assert.Equal(t, http.StatusUnauthorized, first.Original.Status)
	assert.Equal(t, "invalid credential", first.Original.Message)
	assert.Equal(t, "invalid_api_key", first.Original.Code)
	assert.Equal(t, http.StatusTooManyRequests, first.Overridden.Status)
	assert.Equal(t, "服务繁忙，请稍后再试", first.Overridden.Message)
	assert.Nil(t, readAudit(logs[1].Other), "an unmatched error leaves no override audit")
}
