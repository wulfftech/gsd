package repo

import (
	"context"
	"errors"
	"time"

	"donetick.com/core/config"
	cModel "donetick.com/core/internal/circle/model"
	pModel "donetick.com/core/internal/points"
	rModel "donetick.com/core/internal/reward/model"
	"donetick.com/core/logging"
	"gorm.io/gorm"
)

type RewardRepository struct {
	db *gorm.DB
}

func NewRewardRepository(db *gorm.DB, cfg *config.Config) *RewardRepository {
	return &RewardRepository{db: db}
}

func (r *RewardRepository) GetRewards(ctx context.Context, circleID int) ([]*rModel.Reward, error) {
	rewards := make([]*rModel.Reward, 0)
	if err := r.db.WithContext(ctx).
		Where("circle_id = ? AND is_active = true", circleID).
		Order("points ASC").
		Find(&rewards).Error; err != nil {
		return rewards, err
	}
	return rewards, nil
}

func (r *RewardRepository) GetRewardByID(ctx context.Context, circleID int, rewardID int) (*rModel.Reward, error) {
	var reward rModel.Reward
	if err := r.db.WithContext(ctx).
		Where("id = ? AND circle_id = ? AND is_active = true", rewardID, circleID).
		First(&reward).Error; err != nil {
		return nil, err
	}
	return &reward, nil
}

func (r *RewardRepository) CreateReward(ctx context.Context, reward *rModel.Reward) error {
	reward.CreatedAt = time.Now().UTC()
	reward.UpdatedAt = time.Now().UTC()
	return r.db.WithContext(ctx).Create(reward).Error
}

func (r *RewardRepository) UpdateReward(ctx context.Context, reward *rModel.Reward) error {
	reward.UpdatedAt = time.Now().UTC()
	return r.db.WithContext(ctx).
		Model(&rModel.Reward{}).
		Where("id = ? AND circle_id = ?", reward.ID, reward.CircleID).
		Updates(reward).Error
}

func (r *RewardRepository) DeleteReward(ctx context.Context, circleID int, rewardID int) error {
	return r.db.WithContext(ctx).
		Model(&rModel.Reward{}).
		Where("id = ? AND circle_id = ?", rewardID, circleID).
		Update("is_active", false).Error
}

func (r *RewardRepository) RedeemReward(ctx context.Context, circleID int, userID int, reward *rModel.Reward) error {
	logger := logging.FromContext(ctx)
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Read current balance inside the transaction to avoid race conditions
		var uc cModel.UserCircle
		if err := tx.Where("user_id = ? AND circle_id = ?", userID, circleID).First(&uc).Error; err != nil {
			return err
		}
		if uc.Points < reward.Points {
			return errors.New("insufficient points")
		}

		// Deduct points from balance
		if err := tx.Model(&cModel.UserCircle{}).
			Where("user_id = ? AND circle_id = ?", userID, circleID).
			Update("points", gorm.Expr("points - ?", reward.Points)).Error; err != nil {
			logger.Error("Error deducting points", err)
			return err
		}

		// Record the redemption
		now := time.Now().UTC()
		redemption := &rModel.RewardRedemption{
			RewardID:   reward.ID,
			CircleID:   circleID,
			UserID:     userID,
			Points:     reward.Points,
			Status:     "pending",
			RedeemedAt: now,
		}
		if err := tx.Create(redemption).Error; err != nil {
			logger.Error("Error creating reward redemption", err)
			return err
		}

		// Log to points history
		if err := tx.Create(&pModel.PointsHistory{
			Action:    pModel.PointsHistoryActionRedeem,
			CircleID:  circleID,
			UserID:    userID,
			Points:    reward.Points,
			CreatedAt: now,
			CreatedBy: userID,
		}).Error; err != nil {
			logger.Error("Error creating points history", err)
			return err
		}

		return nil
	})
}

func (r *RewardRepository) GetRedemptions(ctx context.Context, circleID int) ([]*rModel.RewardRedemptionDetail, error) {
	redemptions := make([]*rModel.RewardRedemptionDetail, 0)
	if err := r.db.WithContext(ctx).
		Raw(`SELECT reward_redemptions.*, rewards.name AS reward_name, users.username, users.display_name
			FROM reward_redemptions
			LEFT JOIN rewards ON rewards.id = reward_redemptions.reward_id
			LEFT JOIN users ON users.id = reward_redemptions.user_id
			WHERE reward_redemptions.circle_id = ?
			ORDER BY reward_redemptions.redeemed_at DESC`, circleID).
		Scan(&redemptions).Error; err != nil {
		return nil, err
	}
	return redemptions, nil
}

func (r *RewardRepository) FulfillRedemption(ctx context.Context, circleID int, redemptionID int) error {
	now := time.Now().UTC()
	result := r.db.WithContext(ctx).
		Model(&rModel.RewardRedemption{}).
		Where("id = ? AND circle_id = ? AND status = 'pending'", redemptionID, circleID).
		Updates(map[string]interface{}{
			"status":       "fulfilled",
			"fulfilled_at": now,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("redemption not found or already fulfilled")
	}
	return nil
}
