package repo

import (
	"context"
	"errors"
	"time"

	cModel "donetick.com/core/internal/circle/model"
	pModel "donetick.com/core/internal/points"
	"gorm.io/gorm"
)

// ErrNothingToPay is returned when a member's balance is already zero.
var ErrNothingToPay = errors.New("nothing to pay out")

// PayoutPoints pays a member's whole balance out (with pocket money) and resets
// it to zero, recording how much was paid. Returns the amount paid.
func (r *CircleRepository) PayoutPoints(c context.Context, circleID, userID, createdBy int) (int, error) {
	paid := 0
	err := r.db.WithContext(c).Transaction(func(tx *gorm.DB) error {
		var uc cModel.UserCircle
		if err := tx.Where("user_id = ? AND circle_id = ?", userID, circleID).First(&uc).Error; err != nil {
			return err
		}
		if uc.Points <= 0 {
			return ErrNothingToPay
		}
		paid = uc.Points
		res := tx.Model(&cModel.UserCircle{}).
			Where("user_id = ? AND circle_id = ? AND points = ?", userID, circleID, paid).
			Update("points", 0)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return errors.New("balance changed while paying out; try again")
		}
		return tx.Create(&pModel.PointsHistory{
			Action:    pModel.PointsHistoryActionPayout,
			CircleID:  circleID,
			UserID:    userID,
			Points:    paid,
			CreatedAt: time.Now().UTC(),
			CreatedBy: createdBy,
		}).Error
	})
	return paid, err
}
