package common

import (
	"fmt"
	"strconv"
	"strings"
)

// QuotaConversion provides utilities for converting between different cost units
// This helps users understand costs in multiple formats:
// - Quota (system internal unit)
// - Tokens (approximate input tokens)
// - USD (US dollars)
// - CNY (Chinese Yuan)

// QuotaToUSD converts quota to USD
// Formula: USD = quota / QuotaPerUnit
// Example: 500,000 quota = $1.00 USD
func QuotaToUSD(quota float64) float64 {
	if QuotaPerUnit == 0 {
		return 0
	}
	return quota / QuotaPerUnit
}

// QuotaToCNY converts quota to CNY
// Formula: CNY = (quota / QuotaPerUnit) * USDToCNYRate
// Example: 500,000 quota = $1.00 = ¥7.2 CNY (at default rate)
func QuotaToCNY(quota float64) float64 {
	return QuotaToUSD(quota) * USDToCNYRate
}

// QuotaToTokens converts quota to approximate input tokens
// For most Claude models, 1 quota ≈ 1 input token
// Note: This is an approximation as actual token costs vary by model
func QuotaToTokens(quota float64) int {
	return int(quota)
}

// USDToQuota converts USD to quota
// Formula: quota = USD * QuotaPerUnit
func USDToQuota(usd float64) float64 {
	return usd * QuotaPerUnit
}

// CNYToQuota converts CNY to quota
// Formula: quota = (CNY / USDToCNYRate) * QuotaPerUnit
func CNYToQuota(cny float64) float64 {
	if USDToCNYRate == 0 {
		return 0
	}
	return (cny / USDToCNYRate) * QuotaPerUnit
}

// TokensToQuota converts approximate tokens to quota
// For most Claude models, 1 input token ≈ 1 quota
func TokensToQuota(tokens int) float64 {
	return float64(tokens)
}

// FormatQuotaWithUnit formats quota value with specified unit
// Supported units: "quota", "usd", "cny", "tokens"
func FormatQuotaWithUnit(quota float64, unit string) string {
	switch unit {
	case "usd":
		return "$" + formatFloat(QuotaToUSD(quota), 4)
	case "cny":
		return "¥" + formatFloat(QuotaToCNY(quota), 4)
	case "tokens":
		return formatInt(QuotaToTokens(quota)) + " tokens"
	case "quota":
		fallthrough
	default:
		return formatFloat(quota, 2) + " 额度"
	}
}

// formatFloat formats float with specified decimal places
func formatFloat(value float64, decimals int) string {
	format := "%." + strconv.Itoa(decimals) + "f"
	return fmt.Sprintf(format, value)
}

// formatInt formats integer with thousand separators
func formatInt(value int) string {
	str := strconv.Itoa(value)
	if len(str) <= 3 {
		return str
	}

	// Add thousand separators
	var result strings.Builder
	for i, digit := range str {
		if i > 0 && (len(str)-i)%3 == 0 {
			result.WriteRune(',')
		}
		result.WriteRune(digit)
	}
	return result.String()
}

// GetCostUnitLabel returns the display label for a unit
func GetCostUnitLabel(unit string) string {
	switch unit {
	case "usd":
		return "USD ($)"
	case "cny":
		return "CNY (¥)"
	case "tokens":
		return "Tokens"
	case "quota":
		return "额度"
	default:
		return "Unknown"
	}
}

// GetCostUnitDescription returns a description for a unit
func GetCostUnitDescription(unit string) string {
	switch unit {
	case "usd":
		return "美元 (US Dollar)"
	case "cny":
		return "人民币 (Chinese Yuan)"
	case "tokens":
		return "约等于输入token数"
	case "quota":
		return "系统内部额度单位"
	default:
		return ""
	}
}