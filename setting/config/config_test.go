package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testConfigWithMap struct {
	Modes map[string]string `json:"modes"`
	Exprs map[string]string `json:"exprs"`
	Name  string            `json:"name"`
}

func TestUpdateConfigFromMap_MapReplacement(t *testing.T) {
	cfg := &testConfigWithMap{
		Modes: map[string]string{
			"model-a": "tiered_expr",
			"model-b": "tiered_expr",
		},
		Exprs: map[string]string{
			"model-a": "p * 5 + c * 25",
			"model-b": "p * 10 + c * 50",
		},
		Name: "billing",
	}

	// Simulate removing model-a: new value only has model-b
	err := UpdateConfigFromMap(cfg, map[string]string{
		"modes": `{"model-b": "tiered_expr"}`,
		"exprs": `{"model-b": "p * 10 + c * 50"}`,
	})
	if err != nil {
		t.Fatalf("UpdateConfigFromMap failed: %v", err)
	}

	if _, ok := cfg.Modes["model-a"]; ok {
		t.Errorf("Modes still contains model-a after it was removed from the update; got %v", cfg.Modes)
	}
	if _, ok := cfg.Exprs["model-a"]; ok {
		t.Errorf("Exprs still contains model-a after it was removed from the update; got %v", cfg.Exprs)
	}

	if cfg.Modes["model-b"] != "tiered_expr" {
		t.Errorf("Modes[model-b] = %q, want %q", cfg.Modes["model-b"], "tiered_expr")
	}
	if cfg.Exprs["model-b"] != "p * 10 + c * 50" {
		t.Errorf("Exprs[model-b] = %q, want %q", cfg.Exprs["model-b"], "p * 10 + c * 50")
	}
}

func TestUpdateConfigFromMap_EmptyMapClearsAll(t *testing.T) {
	cfg := &testConfigWithMap{
		Modes: map[string]string{
			"model-a": "tiered_expr",
		},
		Exprs: map[string]string{
			"model-a": "p * 5 + c * 25",
		},
	}

	err := UpdateConfigFromMap(cfg, map[string]string{
		"modes": `{}`,
		"exprs": `{}`,
	})
	if err != nil {
		t.Fatalf("UpdateConfigFromMap failed: %v", err)
	}

	if len(cfg.Modes) != 0 {
		t.Errorf("Modes should be empty after updating with {}, got %v", cfg.Modes)
	}
	if len(cfg.Exprs) != 0 {
		t.Errorf("Exprs should be empty after updating with {}, got %v", cfg.Exprs)
	}
}

func TestUpdateConfigFromMap_ScalarFieldsUnchanged(t *testing.T) {
	cfg := &testConfigWithMap{
		Modes: map[string]string{"m": "v"},
		Name:  "old",
	}

	err := UpdateConfigFromMap(cfg, map[string]string{
		"name": "new",
	})
	if err != nil {
		t.Fatalf("UpdateConfigFromMap failed: %v", err)
	}

	if cfg.Name != "new" {
		t.Errorf("Name = %q, want %q", cfg.Name, "new")
	}
	// modes was not in configMap, should remain unchanged
	if cfg.Modes["m"] != "v" {
		t.Errorf("Modes should be unchanged, got %v", cfg.Modes)
	}
}

// testPaymentLikeConfig mirrors PaymentSetting: JSON collections whose values
// arrive as strings from the options table and are cleared from the admin UI
// by submitting an empty string.
type testPaymentLikeConfig struct {
	AmountOptions  []int           `json:"amount_options"`
	AmountDiscount map[int]float64 `json:"amount_discount"`
	AmountBonus    map[int]float64 `json:"amount_bonus"`
}

func TestUpdateConfigFromMap_EmptyStringClearsJSONCollections(t *testing.T) {
	assert := assert.New(t)
	cfg := &testPaymentLikeConfig{
		AmountOptions:  []int{10, 20, 50},
		AmountDiscount: map[int]float64{100: 0.96},
		AmountBonus:    map[int]float64{100: 10},
	}

	err := UpdateConfigFromMap(cfg, map[string]string{
		"amount_options":  "",
		"amount_discount": "",
		"amount_bonus":    "   ",
	})
	require.NoError(t, err)

	assert.Empty(cfg.AmountOptions, "empty string should clear the slice")
	assert.Empty(cfg.AmountDiscount, "empty string should clear the discount map")
	assert.Empty(cfg.AmountBonus, "whitespace-only string should clear the bonus map")
}

func TestUpdateConfigFromMap_EmptyStringThenValidJSONRestores(t *testing.T) {
	assert := assert.New(t)
	cfg := &testPaymentLikeConfig{
		AmountBonus: map[int]float64{100: 10},
	}

	err := UpdateConfigFromMap(cfg, map[string]string{"amount_bonus": ""})
	require.NoError(t, err)
	assert.Empty(cfg.AmountBonus, "empty string should clear the bonus map")

	err = UpdateConfigFromMap(cfg, map[string]string{"amount_bonus": `{"50":5}`})
	require.NoError(t, err)
	assert.Equal(map[int]float64{50: 5}, cfg.AmountBonus, "valid JSON must restore the map")

	err = UpdateConfigFromMap(cfg, map[string]string{"amount_bonus": `{}`})
	require.NoError(t, err)
	assert.Empty(cfg.AmountBonus, "{} should clear the bonus map")
}

func TestUpdateConfigFromMap_InvalidNonEmptyJSONKeepsOldValue(t *testing.T) {
	assert := assert.New(t)
	cfg := &testPaymentLikeConfig{
		AmountBonus: map[int]float64{100: 10},
	}

	err := UpdateConfigFromMap(cfg, map[string]string{"amount_bonus": "{bad json"})
	require.NoError(t, err)
	assert.Equal(map[int]float64{100: 10}, cfg.AmountBonus, "invalid non-empty JSON must not clobber the old value")
}
