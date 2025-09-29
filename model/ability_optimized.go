package model

import (
	"errors"
	"fmt"
	"math/rand"
	"one-api/common"

	"gorm.io/gorm"
)

// ChannelWithAbility represents a channel with its ability information
type ChannelWithAbility struct {
	Channel
	AbilityWeight   uint   `gorm:"column:ability_weight"`
	AbilityPriority *int64 `gorm:"column:ability_priority"`
	AbilityEnabled  bool   `gorm:"column:ability_enabled"`
}

// GetRandomSatisfiedChannelOptimized is the optimized version that eliminates N+1 queries
func GetRandomSatisfiedChannelOptimized(group string, model string, retry int) (*Channel, error) {
	priority, err := getTargetPriority(group, model, retry)
	if err != nil {
		return nil, err
	}

	// Single JOIN query to get channels with abilities - eliminates N+1 pattern
	var channelsWithAbilities []ChannelWithAbility
	query := buildOptimizedChannelQuery(group, model, priority)

	err = query.Scan(&channelsWithAbilities).Error
	if err != nil {
		return nil, err
	}

	if len(channelsWithAbilities) == 0 {
		return nil, nil
	}

	// Optimized weight-based selection
	selectedChannel := selectChannelByWeight(channelsWithAbilities)
	return &selectedChannel.Channel, nil
}

// getTargetPriority determines the priority level for the retry attempt
func getTargetPriority(group string, model string, retry int) (*int64, error) {
	if retry == 0 {
		// For retry = 0, use max priority (subquery optimization)
		return nil, nil
	}

	// For retry > 0, get specific priority level
	var priorities []int64
	groupCol := "`group`"
	if common.UsingPostgreSQL {
		groupCol = `"group"`
	}

	err := DB.Model(&Ability{}).
		Select("DISTINCT(priority)").
		Where(groupCol+" = ? AND model = ? AND enabled = ?", group, model, true).
		Order("priority DESC").
		Pluck("priority", &priorities).Error

	if err != nil {
		return nil, err
	}

	if len(priorities) == 0 {
		return nil, errors.New("no available channels for specified group and model")
	}

	var targetPriority int64
	if retry >= len(priorities) {
		targetPriority = priorities[len(priorities)-1]
	} else {
		targetPriority = priorities[retry]
	}

	return &targetPriority, nil
}

// buildOptimizedChannelQuery constructs the optimized single-query channel selection
func buildOptimizedChannelQuery(group string, model string, priority *int64) *gorm.DB {
	groupCol := "`group`"
	if common.UsingPostgreSQL {
		groupCol = `"group"`
	}

	// Build the optimized JOIN query
	query := DB.Table("channels c").
		Select(`c.*,
			a.weight as ability_weight,
			a.priority as ability_priority,
			a.enabled as ability_enabled`).
		Joins("INNER JOIN abilities a ON c.id = a.channel_id").
		Where("a."+groupCol+" = ? AND a.model = ? AND a.enabled = ? AND c.status = ?",
			group, model, true, common.ChannelStatusEnabled)

	if priority == nil {
		// Use max priority subquery for retry = 0
		maxPrioritySubQuery := DB.Model(&Ability{}).
			Select("MAX(priority)").
			Where("a."+groupCol+" = ? AND a.model = ? AND a.enabled = ?", group, model, true)
		query = query.Where("a.priority = (?)", maxPrioritySubQuery)
	} else {
		// Use specific priority for retry > 0
		query = query.Where("a.priority = ?", *priority)
	}

	// Order by weight DESC to prioritize higher weights
	// The new composite index (group, model, enabled, priority, weight) makes this extremely fast
	return query.Order("a.weight DESC")
}

// selectChannelByWeight implements optimized weight-based channel selection
func selectChannelByWeight(channels []ChannelWithAbility) *ChannelWithAbility {
	if len(channels) == 1 {
		return &channels[0]
	}

	// Calculate total weight (optimized for large channel lists)
	totalWeight := uint(0)
	for i := range channels {
		totalWeight += channels[i].AbilityWeight + 10
	}

	if totalWeight == 0 {
		// Fallback to random selection if all weights are 0
		return &channels[rand.Intn(len(channels))]
	}

	// Weighted random selection
	randomWeight := rand.Intn(int(totalWeight))
	currentWeight := 0

	for i := range channels {
		currentWeight += int(channels[i].AbilityWeight) + 10
		if currentWeight > randomWeight {
			return &channels[i]
		}
	}

	// Fallback (should not reach here)
	return &channels[len(channels)-1]
}

// GetRandomSatisfiedChannelWithFallback provides backward compatibility and fallback
func GetRandomSatisfiedChannelWithFallback(group string, model string, retry int) (*Channel, error) {
	// Try optimized version first
	channel, err := GetRandomSatisfiedChannelOptimized(group, model, retry)
	if err == nil && channel != nil {
		return channel, nil
	}

	// Log the optimization attempt
	if common.DebugEnabled {
		common.SysLog(fmt.Sprintf("Optimized channel selection failed for group=%s, model=%s, retry=%d: %v",
			group, model, retry, err))
	}

	// Fallback to original implementation
	return GetRandomSatisfiedChannelLegacy(group, model, retry)
}

// GetRandomSatisfiedChannelLegacy is the original implementation (renamed for fallback)
func GetRandomSatisfiedChannelLegacy(group string, model string, retry int) (*Channel, error) {
	var abilities []Ability

	var err error = nil
	channelQuery, err := getChannelQuery(group, model, retry)
	if err != nil {
		return nil, err
	}
	if common.UsingSQLite || common.UsingPostgreSQL {
		err = channelQuery.Order("weight DESC").Find(&abilities).Error
	} else {
		err = channelQuery.Order("weight DESC").Find(&abilities).Error
	}
	if err != nil {
		return nil, err
	}
	channel := Channel{}
	if len(abilities) > 0 {
		// Randomly choose one
		weightSum := uint(0)
		for _, ability_ := range abilities {
			weightSum += ability_.Weight + 10
		}
		// Randomly choose one
		weight := common.GetRandomInt(int(weightSum))
		for _, ability_ := range abilities {
			weight -= int(ability_.Weight) + 10
			//log.Printf("weight: %d, ability weight: %d", weight, *ability_.Weight)
			if weight <= 0 {
				channel.Id = ability_.ChannelId
				break
			}
		}
	} else {
		return nil, nil
	}
	err = DB.First(&channel, "id = ?", channel.Id).Error
	return &channel, err
}

// calculateAverage helper function
func calculateAverage(times []int64) float64 {
	if len(times) == 0 {
		return 0
	}

	total := int64(0)
	for _, t := range times {
		total += t
	}

	return float64(total) / float64(len(times))
}