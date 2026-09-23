package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
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
