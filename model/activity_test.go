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
	"os"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestActivityDatabaseMatrix(t *testing.T) {
	for _, dialect := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			var dialector gorm.Dialector
			switch dialect {
			case "sqlite":
				dialector = sqlite.Open(":memory:")
			case "mysql":
				dsn := os.Getenv("TEST_MYSQL_DSN")
				if dsn == "" {
					t.Skip("TEST_MYSQL_DSN is not configured")
				}
				dialector = mysql.Open(dsn)
			case "postgres":
				dsn := os.Getenv("TEST_POSTGRES_DSN")
				if dsn == "" {
					t.Skip("TEST_POSTGRES_DSN is not configured")
				}
				dialector = postgres.Open(dsn)
			}

			db, err := gorm.Open(dialector, &gorm.Config{
				NamingStrategy: schema.NamingStrategy{TablePrefix: "activity_test_"},
			})
			require.NoError(t, err)
			sqlDB, err := db.DB()
			require.NoError(t, err)
			sqlDB.SetMaxOpenConns(1)

			previousDB := DB
			DB = db
			t.Cleanup(func() {
				DB = previousDB
				require.NoError(t, db.Migrator().DropTable(&Activity{}))
				require.NoError(t, sqlDB.Close())
			})
			require.NoError(t, db.AutoMigrate(&Activity{}))

			first := Activity{ActivityDate: "2026-12-31", Title: "Year end", Description: "Last day"}
			require.NoError(t, first.Insert())
			second := Activity{ActivityDate: "2026-01-05", Title: "Kickoff", Description: "First event"}
			require.NoError(t, second.Insert())

			activities, err := GetAllActivities()
			require.NoError(t, err)
			require.Len(t, activities, 2)
			assert.Equal(t, "2026-01-05", activities[0].ActivityDate)

			second.Title = "Updated kickoff"
			require.NoError(t, second.Update())
			stored, err := GetActivityByID(second.Id)
			require.NoError(t, err)
			assert.Equal(t, "Updated kickoff", stored.Title)
			assert.NotZero(t, stored.CreatedTime)

			require.NoError(t, DeleteActivityByID(second.Id))
			activities, err = GetAllActivities()
			require.NoError(t, err)
			require.Len(t, activities, 1)
			assert.Equal(t, first.Id, activities[0].Id)
		})
	}
}
