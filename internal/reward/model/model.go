package model

import "time"

type Reward struct {
	ID          int       `json:"id" gorm:"primary_key"`
	CircleID    int       `json:"circleId" gorm:"column:circle_id;index"`
	Name        string    `json:"name" gorm:"column:name"`
	Description string    `json:"description" gorm:"column:description"`
	Points      int       `json:"points" gorm:"column:points"`
	IsActive    bool      `json:"isActive" gorm:"column:is_active;default:true"`
	CreatedBy   int       `json:"createdBy" gorm:"column:created_by"`
	CreatedAt   time.Time `json:"createdAt" gorm:"column:created_at"`
	UpdatedAt   time.Time `json:"updatedAt" gorm:"column:updated_at"`
}

type RewardRedemption struct {
	ID          int       `json:"id" gorm:"primary_key"`
	RewardID    int       `json:"rewardId" gorm:"column:reward_id;index"`
	CircleID    int       `json:"circleId" gorm:"column:circle_id;index"`
	UserID      int       `json:"userId" gorm:"column:user_id;index"`
	Points      int       `json:"points" gorm:"column:points"`
	Status      string    `json:"status" gorm:"column:status;default:pending"` // pending | fulfilled
	RedeemedAt  time.Time `json:"redeemedAt" gorm:"column:redeemed_at"`
	FulfilledAt *time.Time `json:"fulfilledAt" gorm:"column:fulfilled_at"`
}

type RewardRedemptionDetail struct {
	RewardRedemption
	RewardName  string `json:"rewardName" gorm:"column:reward_name"`
	Username    string `json:"username" gorm:"column:username"`
	DisplayName string `json:"displayName" gorm:"column:display_name"`
}
