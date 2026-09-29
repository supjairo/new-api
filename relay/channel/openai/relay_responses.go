package openai

import (
	"fmt"
	"io"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

func OaiResponsesHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	defer service.CloseResponseBodyGracefully(resp)

	// read response body
	var responsesResponse dto.OpenAIResponsesResponse
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}
	err = common.Unmarshal(responseBody, &responsesResponse)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	if oaiError := responsesResponse.GetOpenAIError(); oaiError != nil && oaiError.Type != "" {
		return nil, types.WithOpenAIError(*oaiError, resp.StatusCode)
	}

	info.ObserveResponseModel(responsesResponse.Model)
	responseBody = rewriteSGLangResponsesCreatedAt(info, responseBody, "created_at", responsesResponse.CreatedAt)

	// 写入新的 response body
	service.IOCopyBytesGracefully(c, resp, responseBody)

	// compute usage
	usage := &dto.Usage{}
	service.ApplyResponsesUsage(usage, responsesResponse.Usage)
	// Count actual tool invocations from Output (not tool declarations).
	for _, output := range responsesResponse.Output {
		switch output.Type {
		case dto.BuildInCallWebSearchCall:
			info.CountBillableToolCall(dto.BuildInCallWebSearchCall, "")
		case dto.BuildInCallFileSearchCall:
			info.CountBillableToolCall(dto.BuildInCallFileSearchCall, "")
		case dto.BuildInCallFunctionCall:
			info.CountBillableToolCall(dto.BuildInCallFunctionCall, output.Name)
		}
	}

	imageCounter := &relaycommon.ImageGenerationCallCounter{}
	if !relaycommon.IsNonBillableResponsesStatus(responsesResponse.Status) {
		for i := range responsesResponse.Output {
			idx := i
			imageCounter.Observe(&responsesResponse.Output[i], &idx)
		}
	}
	imageCounter.Commit(info)

	return usage, nil
}

func OaiResponsesStreamHandler(c *gin.Context, info *relaycommon.RelayInfo, resp *http.Response) (*dto.Usage, *types.NewAPIError) {
	if resp == nil || resp.Body == nil {
		logger.LogError(c, "invalid response or response body")
		return nil, types.NewError(fmt.Errorf("invalid response"), types.ErrorCodeBadResponse)
	}

	defer service.CloseResponseBodyGracefully(resp)

	accumulator := service.NewResponsesUsageAccumulator(info)

	helper.StreamScannerHandler(c, resp, info, func(data string, sr *helper.StreamResult) {

		// 检查当前数据是否包含 completed 状态和 usage 信息
		var streamResponse dto.ResponsesStreamResponse
		if err := common.UnmarshalJsonStr(data, &streamResponse); err != nil {
			logger.LogError(c, "failed to unmarshal stream response: "+err.Error())
			sr.Error(err)
			return
		}
		if streamResponse.Response != nil {
			data = string(rewriteSGLangResponsesCreatedAt(info, []byte(data), "response.created_at", streamResponse.Response.CreatedAt))
		}
		data, streamResponse = applyResponsesStreamErrorOverride(c, info, streamResponse, data)
		if data == "" {
			return
		}
		sendResponsesStreamData(c, streamResponse, data)
		accumulator.Observe(&streamResponse)
	})

	common.SetContextKey(c, constant.ContextKeyResponseStreamStatus, info.StreamStatus)
	info.StreamStatus.RequireTerminal()
	return accumulator.Finish(), nil
}

// responsesStreamErrorEvents lists the event types the stream error override
// may touch. Restricting evaluation to error events keeps a catch-all
// fallback rule (empty match) from rewriting normal content events such as
// response.output_text.delta.
var responsesStreamErrorEvents = map[string]struct{}{
	"response.failed": {},
	"response.error":  {},
	"error":           {},
}

// applyResponsesStreamErrorOverride inspects one upstream event against the
// channel's stream error override rules and returns the rewritten raw data
// plus event when a rule fires. A rule that deletes the event type drops the
// event entirely (empty data). Only the client-facing payload changes; the
// usage accumulator keeps observing the overridden event so settlement stays
// consistent with what the client received.
func applyResponsesStreamErrorOverride(c *gin.Context, info *relaycommon.RelayInfo, streamResponse dto.ResponsesStreamResponse, data string) (string, dto.ResponsesStreamResponse) {
	if _, isErrEvent := responsesStreamErrorEvents[streamResponse.Type]; !isErrEvent {
		return data, streamResponse
	}
	rulesStr := service.PrepareStreamErrorOverrideRules(c)
	if rulesStr == "" {
		return data, streamResponse
	}
	tree := service.StreamErrorEvent(streamResponse.Type, streamEventPayload(streamResponse))
	rewritten := service.ApplyStreamErrorOverride(tree, rulesStr)
	if rewritten == nil {
		return data, streamResponse
	}
	newType, _ := rewritten["event"].(string)
	if newType == "" {
		// The rule deleted the event field; treat it as a dropped event.
		logger.LogInfo(c, "stream error override dropped event: "+streamResponse.Type)
		return "", streamResponse
	}
	// The tree names the event "event"; the Responses wire format uses "type".
	wire := make(map[string]any, len(rewritten))
	for key, value := range rewritten {
		if key == "event" {
			wire["type"] = value
			continue
		}
		wire[key] = value
	}
	patch, err := common.Marshal(wire)
	if err != nil {
		return data, streamResponse
	}
	var patched dto.ResponsesStreamResponse
	if uerr := common.Unmarshal(patch, &patched); uerr != nil {
		logger.LogError(c, "stream error override re-decode failed: "+uerr.Error())
		return data, streamResponse
	}
	patched.Type = newType
	logger.LogInfo(c, fmt.Sprintf("stream error override applied: event=%s -> %s", streamResponse.Type, newType))
	return string(patch), patched
}

// streamEventPayload renders the parsed event as the generic tree fields the
// rules address: the error details merged under a nested "error" object (from
// the response object's error and/or the flat code/message/param fields) plus
// the response status for response.failed style events.
func streamEventPayload(streamResponse dto.ResponsesStreamResponse) map[string]any {
	payload := map[string]any{}
	errObj := map[string]any{}
	if streamResponse.Response != nil {
		if oaiErr := streamResponse.Response.GetOpenAIError(); oaiErr != nil {
			if oaiErr.Message != "" {
				errObj["message"] = oaiErr.Message
			}
			if oaiErr.Type != "" {
				errObj["type"] = oaiErr.Type
			}
			if oaiErr.Param != "" {
				errObj["param"] = oaiErr.Param
			}
			if oaiErr.Code != nil {
				errObj["code"] = oaiErr.Code
			}
		}
		if status := string(streamResponse.Response.Status); status != "" && status != "null" {
			payload["response"] = map[string]any{"status": status}
		}
	}
	if streamResponse.Code != "" {
		errObj["code"] = streamResponse.Code
	}
	if streamResponse.Message != "" {
		errObj["message"] = streamResponse.Message
	}
	if streamResponse.Param != "" {
		errObj["param"] = streamResponse.Param
	}
	if len(errObj) > 0 {
		payload["error"] = errObj
	}
	return payload
}

func rewriteSGLangResponsesCreatedAt(info *relaycommon.RelayInfo, payload []byte, path string, createdAt dto.IntValue) []byte {
	if info.GetChannelType() != constant.ChannelTypeSGLang {
		return payload
	}
	if !gjson.GetBytes(payload, path).Exists() {
		return payload
	}
	patched, err := sjson.SetBytes(payload, path, int(createdAt))
	if err != nil {
		return payload
	}
	return patched
}
