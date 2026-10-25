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
package model

import (
	"github.com/QuantumNous/new-api/common"
)

// Activity is a dated item shown on the public activity calendar.
// The date is stored as a YYYY-MM-DD string so ordering and range queries
// behave the same on SQLite, MySQL and PostgreSQL.
type Activity struct {
	Id           int    `json:"id" gorm:"primaryKey"`
	ActivityDate string `json:"date" gorm:"column:activity_date;size:10;index;not null"`
	Title        string `json:"title" gorm:"size:200;not null"`
	Description  string `json:"description" gorm:"type:text"`
	CreatedTime  int64  `json:"created_time" gorm:"bigint"`
	UpdatedTime  int64  `json:"updated_time" gorm:"bigint"`
}

func GetAllActivities() ([]*Activity, error) {
	var activities []*Activity
	err := DB.Model(&Activity{}).
		Order("activity_date ASC").
		Order("id ASC").
		Find(&activities).Error
	return activities, err
}

func GetActivityByID(id int) (*Activity, error) {
	var activity Activity
	if err := DB.First(&activity, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &activity, nil
}

func (a *Activity) Insert() error {
	now := common.GetTimestamp()
	a.CreatedTime = now
	a.UpdatedTime = now
	return DB.Create(a).Error
}

func (a *Activity) Update() error {
	a.UpdatedTime = common.GetTimestamp()
	return DB.Save(a).Error
}

func DeleteActivityByID(id int) error {
	return DB.Delete(&Activity{}, id).Error
}
