package openai

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newStreamOverrideTestContext(t *testing.T, rules string) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	if rules != "" {
		common.SetContextKey(c, constant.ContextKeyChannelStreamErrorOverride, rules)
	}
	return c
}

func TestApplyResponsesStreamErrorOverride(t *testing.T) {
	fallback := `[{"match":{},"override":{"error.message":"服务繁忙，请稍后再试","error.code":"server_busy"}}]`

	t.Run("fallback rule rewrites a failed event", func(t *testing.T) {
		c := newStreamOverrideTestContext(t, fallback)
		event := dto.ResponsesStreamResponse{
			Type:    "response.failed",
			Code:    "stream_disconnected",
			Message: "stream disconnected before completion",
		}

		data, patched := applyResponsesStreamErrorOverride(c, nil, event, `{"type":"response.failed","code":"stream_disconnected","message":"stream disconnected before completion"}`)

		require.NotEmpty(t, data)
		require.Equal(t, "response.failed", patched.Type)
		var wire map[string]any
		require.NoError(t, common.UnmarshalJsonStr(data, &wire))
		require.Equal(t, "response.failed", wire["type"])
		errObj, ok := wire["error"].(map[string]any)
		require.True(t, ok, "rewritten data carries a nested error object: %s", data)
		assert.Equal(t, "服务繁忙，请稍后再试", errObj["message"])
		assert.Equal(t, "server_busy", errObj["code"])
	})

	t.Run("fallback rule leaves normal events untouched", func(t *testing.T) {
		c := newStreamOverrideTestContext(t, fallback)
		raw := `{"type":"response.output_text.delta","delta":"hello"}`
		event := dto.ResponsesStreamResponse{Type: "response.output_text.delta", Delta: "hello"}

		data, patched := applyResponsesStreamErrorOverride(c, nil, event, raw)

		assert.Equal(t, raw, data)
		assert.Equal(t, "response.output_text.delta", patched.Type)
		assert.Empty(t, patched.Message)
	})

	t.Run("no rules keeps error events untouched", func(t *testing.T) {
		c := newStreamOverrideTestContext(t, "")
		raw := `{"type":"response.failed","code":"boom","message":"upstream gone"}`
		event := dto.ResponsesStreamResponse{Type: "response.failed", Code: "boom", Message: "upstream gone"}

		data, patched := applyResponsesStreamErrorOverride(c, nil, event, raw)

		assert.Equal(t, raw, data)
		assert.Equal(t, "upstream gone", patched.Message)
	})

	t.Run("specific rule wins over fallback by order", func(t *testing.T) {
		rules := `[{"match":{"event":"response.failed","error.code":"context_too_large"},"override":{"error.message":"上下文过长，请缩短后重试"}},` + fallback[1:]
		c := newStreamOverrideTestContext(t, rules)
		event := dto.ResponsesStreamResponse{Type: "response.failed", Code: "stream_disconnected", Message: "gone"}

		data, patched := applyResponsesStreamErrorOverride(c, nil, event, `{"type":"response.failed"}`)

		require.Equal(t, "response.failed", patched.Type)
		var wire map[string]any
		require.NoError(t, common.UnmarshalJsonStr(data, &wire))
		errObj := wire["error"].(map[string]any)
		assert.Equal(t, "服务繁忙，请稍后再试", errObj["message"])
		assert.Equal(t, "server_busy", errObj["code"])
	})

	t.Run("synthesized abort event matches fallback", func(t *testing.T) {
		c := newStreamOverrideTestContext(t, fallback)

		tree := service.StreamAbortEvent("timeout", 5, false)
		rewritten := service.ApplyStreamErrorOverride(tree, service.PrepareStreamErrorOverrideRules(c))

		require.NotNil(t, rewritten)
		assert.Equal(t, "服务繁忙，请稍后再试", rewritten["error"].(map[string]any)["message"])
		assert.Equal(t, 5, rewritten["received_events"])
	})
}
