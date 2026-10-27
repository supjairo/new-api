/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetTopUpBonusQuota(t *testing.T) {
	originalDisplayType := operation_setting.GetGeneralSetting().QuotaDisplayType
	originalBonus := operation_setting.GetPaymentSetting().AmountBonus
	t.Cleanup(func() {
		operation_setting.GetGeneralSetting().QuotaDisplayType = originalDisplayType
		operation_setting.GetPaymentSetting().AmountBonus = originalBonus
	})

	testCases := []struct {
		name             string
		quotaDisplayType string
		bonus            map[int]float64
		amount           int64
		expected         int
		expectError      bool
	}{
		{
			name:             "currency display applies bonus percentage to amount",
			quotaDisplayType: operation_setting.QuotaDisplayTypeUSD,
			bonus:            map[int]float64{100: 10},
			amount:           100,
			expected:         int(100 * 0.1 * common.QuotaPerUnit),
		},
		{
			name:             "amount without configured tier gets no bonus",
			quotaDisplayType: operation_setting.QuotaDisplayTypeUSD,
			bonus:            map[int]float64{100: 10},
			amount:           50,
			expected:         0,
		},
		{
			name:             "non-positive percentage gets no bonus",
			quotaDisplayType: operation_setting.QuotaDisplayTypeUSD,
			bonus:            map[int]float64{100: 0},
			amount:           100,
			expected:         0,
		},
		{
			name:             "non-positive amount gets no bonus",
			quotaDisplayType: operation_setting.QuotaDisplayTypeUSD,
			bonus:            map[int]float64{100: 10},
			amount:           0,
			expected:         0,
		},
		{
			name:             "percentage above 100 is rejected",
			quotaDisplayType: operation_setting.QuotaDisplayTypeUSD,
			bonus:            map[int]float64{100: 100.5},
			amount:           100,
			expectError:      true,
		},
		{
			name:             "tokens display normalizes amount before matching tier",
			quotaDisplayType: operation_setting.QuotaDisplayTypeTokens,
			bonus:            map[int]float64{3: 50},
			amount:           int64(common.QuotaPerUnit * 3),
			expected:         int(3 * 0.5 * common.QuotaPerUnit),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			operation_setting.GetGeneralSetting().QuotaDisplayType = tc.quotaDisplayType
			operation_setting.GetPaymentSetting().AmountBonus = tc.bonus

			bonus, err := getTopUpBonusQuota(tc.amount)

			if tc.expectError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.expected, bonus)
		})
	}
}
