package migrations

import (
	"context"

	"donetick.com/core/logging"
	"gorm.io/gorm"
)

// RetirePointsRedeemedLedger20260808 folds the legacy points_redeemed counter into the
// single `points` balance, so `points` uniformly means "current spendable balance"
// everywhere (matching what reward.RedeemReward already assumed). Before this migration,
// the legacy circle "redeem points" flow only ever incremented points_redeemed and left
// points untouched, while the newer Rewards catalog checked/deducted points directly with
// no awareness of points_redeemed — the two could be combined to overspend past a member's
// actual balance. See CODE_REVIEW.md Correctness Finding 5.
type RetirePointsRedeemedLedger20260808 struct{}

func (m RetirePointsRedeemedLedger20260808) ID() string {
	return "20260808b_retire_points_redeemed_ledger"
}

func (m RetirePointsRedeemedLedger20260808) Description() string {
	return "One-time reconciliation: points -= points_redeemed, points_redeemed = 0, so points alone is the spendable balance going forward"
}

func (m RetirePointsRedeemedLedger20260808) Up(ctx context.Context, db *gorm.DB) error {
	log := logging.FromContext(ctx)

	return db.Transaction(func(tx *gorm.DB) error {
		// Only rows with a nonzero points_redeemed need reconciling; everything else is a no-op.
		res := tx.Exec(`UPDATE user_circles SET points = points - points_redeemed, points_redeemed = 0 WHERE points_redeemed != 0`)
		if res.Error != nil {
			log.Errorf("Failed to reconcile points_redeemed into points: %v", res.Error)
			return res.Error
		}
		log.Infof("Reconciled points_redeemed into points for %d user_circles rows", res.RowsAffected)
		return nil
	})
}

func (m RetirePointsRedeemedLedger20260808) Down(ctx context.Context, db *gorm.DB) error {
	// Deliberately not reversible: once points and points_redeemed are folded together,
	// the original split can't be reconstructed (which portion of a later deduction was
	// "new" spend vs. already-reconciled legacy spend is not recoverable from the data).
	// A rollback here would silently corrupt balances rather than restore them, so this
	// is a documented no-op rather than a fake Down.
	return nil
}

func init() {
	Register(RetirePointsRedeemedLedger20260808{})
}
