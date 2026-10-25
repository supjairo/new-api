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
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

const (
	activityDateLayout           = "2006-01-02"
	activityTitleMaxLength       = 100
	activityDescriptionMaxLength = 2000
)

func normalizeActivity(activity *model.Activity) error {
	activity.ActivityDate = strings.TrimSpace(activity.ActivityDate)
	activity.Title = strings.TrimSpace(activity.Title)
	activity.Description = strings.TrimSpace(activity.Description)

	if activity.ActivityDate == "" {
		return common.NewMessage("Activity date is required")
	}
	if _, err := time.Parse(activityDateLayout, activity.ActivityDate); err != nil {
		return common.NewMessage("Activity date must use the YYYY-MM-DD format")
	}
	if activity.Title == "" {
		return common.NewMessage("Activity title is required")
	}
	if utf8.RuneCountInString(activity.Title) > activityTitleMaxLength {
		return common.NewMessage("Activity title cannot exceed {{max}} characters", map[string]any{"max": activityTitleMaxLength})
	}
	if utf8.RuneCountInString(activity.Description) > activityDescriptionMaxLength {
		return common.NewMessage("Activity description cannot exceed {{max}} characters", map[string]any{"max": activityDescriptionMaxLength})
	}
	return nil
}

func GetActivities(c *gin.Context) {
	activities, err := model.GetAllActivities()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, activities)
}

func CreateActivity(c *gin.Context) {
	var activity model.Activity
	if err := c.ShouldBindJSON(&activity); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := normalizeActivity(&activity); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := activity.Insert(); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "activity.create", map[string]any{"id": activity.Id, "date": activity.ActivityDate})
	common.ApiSuccess(c, &activity)
}

func UpdateActivity(c *gin.Context) {
	var payload model.Activity
	if err := c.ShouldBindJSON(&payload); err != nil {
		common.ApiError(c, err)
		return
	}
	if payload.Id <= 0 {
		common.ApiErrorT(c, "Missing activity ID")
		return
	}
	if err := normalizeActivity(&payload); err != nil {
		common.ApiError(c, err)
		return
	}
	existing, err := model.GetActivityByID(payload.Id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	existing.ActivityDate = payload.ActivityDate
	existing.Title = payload.Title
	existing.Description = payload.Description
	if err := existing.Update(); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "activity.update", map[string]any{"id": existing.Id, "date": existing.ActivityDate})
	common.ApiSuccess(c, existing)
}

func DeleteActivity(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.ApiErrorT(c, "Invalid activity ID")
		return
	}
	if err := model.DeleteActivityByID(id); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "activity.delete", map[string]any{"id": id})
	common.ApiSuccess(c, nil)
}
