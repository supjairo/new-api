package service

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"

	"github.com/gin-gonic/gin"
)

// Stream error override rewrites error events that appear inside a streaming
// response before they are forwarded to the client. It shares the generic
// dot-path match/replace engine with ErrorOverride but targets stream events
// instead of the outgoing error response, and never touches billing, retry,
// or channel-disable decisions.
//
// The engine sees one event as a JSON tree:
//
//	{ "event": "response.failed", "error": { "message": "...", "code": "..." }, ... }
//
// For a stream that ended without an explicit error event (silent abort), the
// gateway synthesizes a tree with "event": "stream.aborted" plus the
// StreamStatus facts (end_reason, received_events, has_errors) so rules can
// match transport-level aborts too.
type StreamErrorOverrideRule = ErrorOverrideRule

// ParseStreamErrorOverrideRules decodes the channel's stream override rules.
func ParseStreamErrorOverrideRules(rulesStr string) ([]StreamErrorOverrideRule, bool) {
	return parseErrorOverrideRules(rulesStr)
}

// StreamErrorEvent builds the JSON tree the rules address for one upstream
// stream event. eventType is the protocol event name ("response.failed",
// "error", ...); payload holds the parsed event fields under "error" and the
// top level.
func StreamErrorEvent(eventType string, payload map[string]any) map[string]any {
	tree := map[string]any{"event": eventType}
	for key, value := range payload {
		if key == "event" {
			continue
		}
		tree[key] = value
	}
	return tree
}

// ApplyStreamErrorOverride returns the rewritten tree when the first matching
// rule fires, or nil when no rule matches. The input tree is never mutated;
// the returned tree is a deep copy with the override applied.
func ApplyStreamErrorOverride(tree map[string]any, rulesStr string) map[string]any {
	rules, ok := ParseStreamErrorOverrideRules(rulesStr)
	if !ok {
		return nil
	}
	for i := range rules {
		if !errorOverrideRuleMatches(tree, rules[i].Match) {
			continue
		}
		rewritten := deepCopyOverrideTree(tree)
		applyErrorOverrideTree(rewritten, rules[i].Override)
		return rewritten
	}
	return nil
}

// deepCopyOverrideTree clones maps recursively so rewrites never leak into
// the caller's tree (shared nested maps would otherwise be mutated).
func deepCopyOverrideTree(tree map[string]any) map[string]any {
	out := make(map[string]any, len(tree))
	for key, value := range tree {
		if nested, ok := value.(map[string]any); ok {
			out[key] = deepCopyOverrideTree(nested)
			continue
		}
		out[key] = value
	}
	return out
}

// StreamAbortEvent synthesizes the event tree for a stream that ended without
// an explicit error event. endReason mirrors StreamEndReason values
// (timeout, scanner_error, eof, client_gone, panic, ping_fail).
func StreamAbortEvent(endReason string, receivedEvents int, hasErrors bool) map[string]any {
	tree := map[string]any{
		"event":           "stream.aborted",
		"end_reason":      endReason,
		"received_events": receivedEvents,
		"has_errors":      hasErrors,
	}
	if err := strings.TrimSpace(endReason); err == "" {
		tree["end_reason"] = "eof"
	}
	return tree
}

// PrepareStreamErrorOverrideRules returns the channel's stream override rules
// from the request context.
func PrepareStreamErrorOverrideRules(c *gin.Context) string {
	if c == nil {
		return ""
	}
	return common.GetContextKeyString(c, constant.ContextKeyChannelStreamErrorOverride)
}
