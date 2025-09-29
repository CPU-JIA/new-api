package model

import (
	"fmt"
	"one-api/common"
	"strings"
	"time"

	"gorm.io/gorm"
)

// DatabaseIndex represents a database index configuration
type DatabaseIndex struct {
	TableName   string
	IndexName   string
	Columns     []string
	IsUnique    bool
	IsComposite bool
}

// IndexMigration represents an index migration operation
type IndexMigration struct {
	Version     string
	Description string
	Indexes     []DatabaseIndex
	Applied     bool
	AppliedAt   time.Time
}

// Critical performance indexes for N+1 query optimization
var performanceIndexes = []IndexMigration{
	{
		Version:     "v1.0.0_performance_indexes",
		Description: "Add critical composite indexes for N+1 query optimization",
		Indexes: []DatabaseIndex{
			// Abilities table - Critical for GetRandomSatisfiedChannel performance
			{
				TableName:   "abilities",
				IndexName:   "idx_abilities_group_model_enabled_priority_weight",
				Columns:     []string{"group", "model", "enabled", "priority", "weight"},
				IsComposite: true,
				IsUnique:    false,
			},
			{
				TableName:   "abilities",
				IndexName:   "idx_abilities_channel_enabled",
				Columns:     []string{"channel_id", "enabled"},
				IsComposite: true,
				IsUnique:    false,
			},
			{
				TableName:   "abilities",
				IndexName:   "idx_abilities_tag_enabled",
				Columns:     []string{"tag", "enabled"},
				IsComposite: true,
				IsUnique:    false,
			},
			{
				TableName:   "abilities",
				IndexName:   "idx_abilities_enabled_priority_weight",
				Columns:     []string{"enabled", "priority", "weight"},
				IsComposite: true,
				IsUnique:    false,
			},
			// Channels table - Critical for channel selection and filtering
			{
				TableName:   "channels",
				IndexName:   "idx_channels_status_type_priority",
				Columns:     []string{"status", "type", "priority"},
				IsComposite: true,
				IsUnique:    false,
			},
			{
				TableName:   "channels",
				IndexName:   "idx_channels_status_group",
				Columns:     []string{"status", "group"},
				IsComposite: true,
				IsUnique:    false,
			},
			{
				TableName:   "channels",
				IndexName:   "idx_channels_tag_status",
				Columns:     []string{"tag", "status"},
				IsComposite: true,
				IsUnique:    false,
			},
			{
				TableName:   "channels",
				IndexName:   "idx_channels_type_status",
				Columns:     []string{"type", "status"},
				IsComposite: true,
				IsUnique:    false,
			},
			// Additional indexes for common query patterns
			{
				TableName:   "channels",
				IndexName:   "idx_channels_balance_updated_time",
				Columns:     []string{"balance_updated_time"},
				IsComposite: false,
				IsUnique:    false,
			},
			{
				TableName:   "channels",
				IndexName:   "idx_channels_test_time",
				Columns:     []string{"test_time"},
				IsComposite: false,
				IsUnique:    false,
			},
		},
	},
}

// CreateIndexSQL generates the appropriate CREATE INDEX SQL for different databases
func (idx DatabaseIndex) CreateIndexSQL() string {
	var sql strings.Builder

	// Build column list with proper quoting for different databases
	quotedColumns := make([]string, len(idx.Columns))
	for i, col := range idx.Columns {
		if common.UsingPostgreSQL {
			quotedColumns[i] = fmt.Sprintf(`"%s"`, col)
		} else {
			// MySQL and SQLite
			quotedColumns[i] = fmt.Sprintf("`%s`", col)
		}
	}

	// Handle special column names that are reserved keywords
	columnList := strings.Join(quotedColumns, ", ")

	// Build CREATE INDEX statement
	if idx.IsUnique {
		sql.WriteString("CREATE UNIQUE INDEX IF NOT EXISTS ")
	} else {
		sql.WriteString("CREATE INDEX IF NOT EXISTS ")
	}

	// Quote index name appropriately
	if common.UsingPostgreSQL {
		sql.WriteString(fmt.Sprintf(`"%s"`, idx.IndexName))
	} else {
		sql.WriteString(fmt.Sprintf("`%s`", idx.IndexName))
	}

	sql.WriteString(" ON ")

	// Quote table name appropriately
	if common.UsingPostgreSQL {
		sql.WriteString(fmt.Sprintf(`"%s"`, idx.TableName))
	} else {
		sql.WriteString(fmt.Sprintf("`%s`", idx.TableName))
	}

	sql.WriteString(fmt.Sprintf(" (%s)", columnList))

	return sql.String()
}

// DropIndexSQL generates the appropriate DROP INDEX SQL for different databases
func (idx DatabaseIndex) DropIndexSQL() string {
	if common.UsingMySQL {
		// MySQL syntax: DROP INDEX index_name ON table_name
		return fmt.Sprintf("DROP INDEX `%s` ON `%s`", idx.IndexName, idx.TableName)
	} else if common.UsingPostgreSQL {
		// PostgreSQL syntax: DROP INDEX IF EXISTS index_name
		return fmt.Sprintf(`DROP INDEX IF EXISTS "%s"`, idx.IndexName)
	} else {
		// SQLite syntax: DROP INDEX IF EXISTS index_name
		return fmt.Sprintf("DROP INDEX IF EXISTS `%s`", idx.IndexName)
	}
}

// CheckIndexExists verifies if an index exists in the database
func CheckIndexExists(db *gorm.DB, tableName, indexName string) (bool, error) {
	var count int64

	if common.UsingMySQL {
		// MySQL: Check INFORMATION_SCHEMA.STATISTICS
		err := db.Raw(`
			SELECT COUNT(*)
			FROM INFORMATION_SCHEMA.STATISTICS
			WHERE TABLE_SCHEMA = DATABASE()
			AND TABLE_NAME = ?
			AND INDEX_NAME = ?
		`, tableName, indexName).Scan(&count).Error
		return count > 0, err
	} else if common.UsingPostgreSQL {
		// PostgreSQL: Check pg_indexes
		err := db.Raw(`
			SELECT COUNT(*)
			FROM pg_indexes
			WHERE schemaname = 'public'
			AND tablename = ?
			AND indexname = ?
		`, tableName, indexName).Scan(&count).Error
		return count > 0, err
	} else {
		// SQLite: Check sqlite_master
		err := db.Raw(`
			SELECT COUNT(*)
			FROM sqlite_master
			WHERE type = 'index'
			AND name = ?
		`, indexName).Scan(&count).Error
		return count > 0, err
	}
}

// ApplyPerformanceIndexes creates all critical performance indexes
func ApplyPerformanceIndexes(db *gorm.DB) error {
	if !common.IsMasterNode {
		return nil // Only master node should apply indexes
	}

	common.SysLog("Starting performance index optimization...")

	successCount := 0
	errorCount := 0

	for _, migration := range performanceIndexes {
		common.SysLog(fmt.Sprintf("Applying migration: %s - %s", migration.Version, migration.Description))

		for _, idx := range migration.Indexes {
			// Check if index already exists
			exists, err := CheckIndexExists(db, idx.TableName, idx.IndexName)
			if err != nil {
				common.SysLog(fmt.Sprintf("Error checking index %s: %v", idx.IndexName, err))
				errorCount++
				continue
			}

			if exists {
				common.SysLog(fmt.Sprintf("Index %s already exists, skipping", idx.IndexName))
				continue
			}

			// Create the index
			sql := idx.CreateIndexSQL()
			common.SysLog(fmt.Sprintf("Creating index: %s", idx.IndexName))
			if common.DebugEnabled {
				common.SysLog(fmt.Sprintf("SQL: %s", sql))
			}

			err = db.Exec(sql).Error
			if err != nil {
				common.SysLog(fmt.Sprintf("Failed to create index %s: %v", idx.IndexName, err))
				errorCount++
			} else {
				common.SysLog(fmt.Sprintf("Successfully created index: %s", idx.IndexName))
				successCount++
			}
		}
	}

	common.SysLog(fmt.Sprintf("Performance index optimization completed: %d success, %d errors", successCount, errorCount))

	if errorCount > 0 {
		return fmt.Errorf("encountered %d errors during index creation", errorCount)
	}

	return nil
}

// GetDatabaseIndexInfo returns information about existing indexes for monitoring
func GetDatabaseIndexInfo(db *gorm.DB) (map[string]interface{}, error) {
	info := make(map[string]interface{})

	if common.UsingMySQL {
		var indexes []struct {
			TableName string `gorm:"column:TABLE_NAME"`
			IndexName string `gorm:"column:INDEX_NAME"`
			ColumnName string `gorm:"column:COLUMN_NAME"`
			NonUnique int `gorm:"column:NON_UNIQUE"`
		}

		err := db.Raw(`
			SELECT TABLE_NAME, INDEX_NAME, COLUMN_NAME, NON_UNIQUE
			FROM INFORMATION_SCHEMA.STATISTICS
			WHERE TABLE_SCHEMA = DATABASE()
			AND TABLE_NAME IN ('channels', 'abilities')
			ORDER BY TABLE_NAME, INDEX_NAME, SEQ_IN_INDEX
		`).Scan(&indexes).Error

		if err == nil {
			info["mysql_indexes"] = indexes
		}
	} else if common.UsingPostgreSQL {
		var indexes []struct {
			SchemaName string `gorm:"column:schemaname"`
			TableName string `gorm:"column:tablename"`
			IndexName string `gorm:"column:indexname"`
			IndexDef string `gorm:"column:indexdef"`
		}

		err := db.Raw(`
			SELECT schemaname, tablename, indexname, indexdef
			FROM pg_indexes
			WHERE schemaname = 'public'
			AND tablename IN ('channels', 'abilities')
			ORDER BY tablename, indexname
		`).Scan(&indexes).Error

		if err == nil {
			info["postgresql_indexes"] = indexes
		}
	} else {
		var indexes []struct {
			Name string `gorm:"column:name"`
			TableName string `gorm:"column:tbl_name"`
			SQL string `gorm:"column:sql"`
		}

		err := db.Raw(`
			SELECT name, tbl_name, sql
			FROM sqlite_master
			WHERE type = 'index'
			AND tbl_name IN ('channels', 'abilities')
			ORDER BY tbl_name, name
		`).Scan(&indexes).Error

		if err == nil {
			info["sqlite_indexes"] = indexes
		}
	}

	return info, nil
}

// ValidateIndexPerformance runs basic queries to validate index effectiveness
func ValidateIndexPerformance(db *gorm.DB) map[string]interface{} {
	results := make(map[string]interface{})

	// Test 1: Ability lookup performance (simulates GetRandomSatisfiedChannel)
	start := time.Now()
	var abilityCount int64
	db.Model(&Ability{}).Where("enabled = ? AND priority > ?", true, 0).Count(&abilityCount)
	results["ability_lookup_ms"] = time.Since(start).Milliseconds()
	results["ability_count"] = abilityCount

	// Test 2: Channel status lookup performance
	start = time.Now()
	var channelCount int64
	db.Model(&Channel{}).Where("status = ? AND type > ?", 1, 0).Count(&channelCount)
	results["channel_lookup_ms"] = time.Since(start).Milliseconds()
	results["channel_count"] = channelCount

	// Test 3: Complex join performance (abilities with channels)
	start = time.Now()
	var joinCount int64
	db.Table("abilities").
		Joins("JOIN channels ON abilities.channel_id = channels.id").
		Where("abilities.enabled = ? AND channels.status = ?", true, 1).
		Count(&joinCount)
	results["join_lookup_ms"] = time.Since(start).Milliseconds()
	results["join_count"] = joinCount

	results["validation_timestamp"] = time.Now().Unix()

	return results
}