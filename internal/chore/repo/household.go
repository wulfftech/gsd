package chore

import (
	"context"
	"time"

	chModel "donetick.com/core/internal/chore/model"
	"gorm.io/gorm"
)

// PendingApproval is a chore waiting for a parent, with the tick that put it there.
type PendingApproval struct {
	Chore   *chModel.Chore
	History *chModel.ChoreHistory
}

// GetPendingApprovals lists every active chore in the circle that is waiting
// for approval, each with its latest pending history row.
func (r *ChoreRepository) GetPendingApprovals(c context.Context, circleID int) ([]PendingApproval, error) {
	var chores []*chModel.Chore
	if err := r.db.WithContext(c).
		Where("circle_id = ? AND is_active = ? AND status = ?", circleID, true, chModel.ChoreStatusPendingApproval).
		Preload("Assignees").
		Order("id").
		Find(&chores).Error; err != nil {
		return nil, err
	}
	if len(chores) == 0 {
		return nil, nil
	}
	ids := make([]int, len(chores))
	for i, ch := range chores {
		ids[i] = ch.ID
	}
	var hist []*chModel.ChoreHistory
	if err := r.db.WithContext(c).
		Where("chore_id IN ? AND status = ?", ids, chModel.ChoreHistoryStatusPendingApproval).
		Order("performed_at desc").
		Find(&hist).Error; err != nil {
		return nil, err
	}
	latest := map[int]*chModel.ChoreHistory{}
	for _, h := range hist {
		if _, ok := latest[h.ChoreID]; !ok {
			latest[h.ChoreID] = h
		}
	}
	out := make([]PendingApproval, 0, len(chores))
	for _, ch := range chores {
		out = append(out, PendingApproval{Chore: ch, History: latest[ch.ID]})
	}
	return out, nil
}

// GetHouseholdChores lists the circle's active chores that belong to a
// checkpoint or are paid extras, with assignees and subtasks.
func (r *ChoreRepository) GetHouseholdChores(c context.Context, circleID int) ([]*chModel.Chore, error) {
	var chores []*chModel.Chore
	err := r.db.WithContext(c).
		Where("circle_id = ? AND is_active = ? AND checkpoint IS NOT NULL AND checkpoint != ''", circleID, true).
		Preload("Assignees").
		Preload("SubTasks").
		Order("next_due_date").
		Find(&chores).Error
	return chores, err
}

// GetRotationChores lists the circle's active weekly_rotation chores.
func (r *ChoreRepository) GetRotationChores(c context.Context, circleID int) ([]*chModel.Chore, error) {
	var chores []*chModel.Chore
	q := r.db.WithContext(c).Where("is_active = ? AND assign_strategy = ?", true, chModel.AssignmentStrategyWeeklyRotation)
	if circleID != 0 {
		q = q.Where("circle_id = ?", circleID)
	}
	err := q.Preload("Assignees").Order("id").Find(&chores).Error
	return chores, err
}

// CompletedHistoryDueBetween returns completed history rows, for the given
// chores, whose occurrence was due in [from, to). That is "approved today"
// when from/to bound today.
func (r *ChoreRepository) CompletedHistoryDueBetween(c context.Context, choreIDs []int, from, to time.Time) ([]*chModel.ChoreHistory, error) {
	if len(choreIDs) == 0 {
		return nil, nil
	}
	var hist []*chModel.ChoreHistory
	err := r.db.WithContext(c).
		Where("chore_id IN ? AND status = ? AND due_date >= ? AND due_date < ?",
			choreIDs, chModel.ChoreHistoryStatusCompleted, from.UTC(), to.UTC()).
		Find(&hist).Error
	return hist, err
}

// SetAssignedTo updates only the assignee of a chore.
func (r *ChoreRepository) SetAssignedTo(c context.Context, choreID int, userID *int) error {
	return r.db.WithContext(c).Model(&chModel.Chore{}).Where("id = ?", choreID).Update("assigned_to", userID).Error
}

// GetLatestPendingHistory returns the newest pending history row of a chore.
func (r *ChoreRepository) GetLatestPendingHistory(c context.Context, choreID int) (*chModel.ChoreHistory, error) {
	var h chModel.ChoreHistory
	err := r.db.WithContext(c).
		Where("chore_id = ? AND status = ?", choreID, chModel.ChoreHistoryStatusPendingApproval).
		Order("performed_at desc").
		First(&h).Error
	if err != nil {
		return nil, err
	}
	return &h, nil
}

// WithdrawPending removes a pending tick and returns the chore to "to do",
// leaving its due date, assignee and active flag as they were.
func (r *ChoreRepository) WithdrawPending(c context.Context, choreID int, historyID int) error {
	return r.db.WithContext(c).Transaction(func(tx *gorm.DB) error {
		res := tx.Where("id = ? AND chore_id = ? AND status = ?", historyID, choreID, chModel.ChoreHistoryStatusPendingApproval).
			Delete(&chModel.ChoreHistory{})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		var left int64
		if err := tx.Model(&chModel.ChoreHistory{}).
			Where("chore_id = ? AND status = ?", choreID, chModel.ChoreHistoryStatusPendingApproval).
			Count(&left).Error; err != nil {
			return err
		}
		if left > 0 {
			return nil
		}
		return tx.Model(&chModel.Chore{}).
			Where("id = ? AND status = ?", choreID, chModel.ChoreStatusPendingApproval).
			Update("status", chModel.ChoreStatusNoStatus).Error
	})
}

// RejectedSince returns the ids, among choreIDs, that had a tick rejected at or
// after since: jobs a parent sent back for another go.
func (r *ChoreRepository) RejectedSince(c context.Context, choreIDs []int, since time.Time) (map[int]bool, error) {
	out := map[int]bool{}
	if len(choreIDs) == 0 {
		return out, nil
	}
	var ids []int
	err := r.db.WithContext(c).Model(&chModel.ChoreHistory{}).
		Where("chore_id IN ? AND status = ? AND performed_at >= ?", choreIDs, chModel.ChoreHistoryStatusRejected, since.UTC()).
		Distinct().Pluck("chore_id", &ids).Error
	for _, id := range ids {
		out[id] = true
	}
	return out, err
}
