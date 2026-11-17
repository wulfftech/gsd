package chore

import (
	"context"
	"errors"
	"fmt"
	"time"

	chModel "donetick.com/core/internal/chore/model"
	uModel "donetick.com/core/internal/user/model"
	"donetick.com/core/logging"
)

// statusError carries the HTTP status a core action failed with.
type statusError struct {
	code int
	msg  string
}

func (e *statusError) Error() string { return e.msg }

func newStatusError(code int, format string, a ...interface{}) error {
	return &statusError{code: code, msg: fmt.Sprintf(format, a...)}
}

// errorStatus maps an error from a core action to an HTTP status and message.
func errorStatus(err error) (int, string) {
	var se *statusError
	if errors.As(err, &se) {
		return se.code, se.msg
	}
	return 500, err.Error()
}

var (
	errNotPending     = newStatusError(400, "Chore is not pending approval")
	errAlreadyPending = newStatusError(409, "Chore is already waiting for approval")
)

// approvePending settles a chore's pending completion: it schedules the next
// occurrence, awards points to whoever did it, and resets subtasks. The caller
// has already checked that approver may approve.
func (h *Handler) approvePending(ctx context.Context, approver *uModel.UserDetails, chore *chModel.Chore) (*chModel.Chore, error) {
	if chore.Status != chModel.ChoreStatusPendingApproval {
		return nil, errNotPending
	}
	allHistory, err := h.choreRepo.GetChoreHistory(ctx, chore.ID)
	if err != nil {
		return nil, fmt.Errorf("fetching history: %w", err)
	}
	var pending *chModel.ChoreHistory
	for _, hist := range allHistory {
		if hist.Status == chModel.ChoreHistoryStatusPendingApproval {
			pending = hist
			break
		}
	}
	if pending == nil || pending.PerformedAt == nil {
		return nil, newStatusError(500, "No pending approval history found")
	}
	next, assignee, err := h.planNext(ctx, chore, *pending.PerformedAt, pending.CompletedBy, allHistory)
	if err != nil {
		return nil, err
	}
	if err := h.choreRepo.ApproveChore(ctx, chore, approver.ID, next, assignee, true); err != nil {
		return nil, fmt.Errorf("approving: %w", err)
	}
	updated, err := h.choreRepo.GetChore(ctx, chore.ID, approver.ID, chore.CircleID)
	if err != nil {
		return nil, fmt.Errorf("reloading chore: %w", err)
	}
	if updated.SubTasks != nil && updated.FrequencyType != chModel.FrequencyTypeOnce {
		if err := h.stRepo.ResetSubtasksCompletion(ctx, updated.ID); err != nil {
			logging.FromContext(ctx).Errorw("Failed to reset subtasks completion", "error", err, "choreID", updated.ID)
		}
	}
	if h.nPlanner != nil {
		h.nPlanner.GenerateNotifications(ctx, updated)
	}
	if h.eventProducer != nil {
		h.eventProducer.ChoreCompleted(ctx, approver.WebhookURL, chore, &approver.User)
	}
	if h.realTimeService != nil {
		h.realTimeService.GetEventBroadcaster().BroadcastChoreCompleted(updated, &approver.User, pending, nil)
	}
	return updated, nil
}

// rejectPending sends a pending completion back to be done again.
func (h *Handler) rejectPending(ctx context.Context, approver *uModel.UserDetails, chore *chModel.Chore, note *string) (*chModel.Chore, error) {
	if chore.Status != chModel.ChoreStatusPendingApproval {
		return nil, errNotPending
	}
	if err := h.choreRepo.RejectChore(ctx, chore.ID, note); err != nil {
		return nil, fmt.Errorf("rejecting: %w", err)
	}
	updated, err := h.choreRepo.GetChore(ctx, chore.ID, approver.ID, chore.CircleID)
	if err != nil {
		return nil, fmt.Errorf("reloading chore: %w", err)
	}
	if h.realTimeService != nil {
		changes := map[string]interface{}{
			"status":    chModel.ChoreStatusNoStatus,
			"updatedBy": approver.ID,
			"updatedAt": time.Now().UTC(),
		}
		h.realTimeService.GetEventBroadcaster().BroadcastChoreUpdated(updated, &approver.User, changes, note)
	}
	return updated, nil
}

// submitForApproval records performer's tick on a chore that needs approval.
// Used by the kiosk, which only ever handles approval-required chores.
func (h *Handler) submitForApproval(ctx context.Context, actor *uModel.UserDetails, chore *chModel.Chore, performer int, at time.Time) (*chModel.Chore, error) {
	if chore.Status == chModel.ChoreStatusPendingApproval {
		return nil, errAlreadyPending
	}
	if !chore.RequireApproval {
		return nil, newStatusError(400, "Only jobs that need a parent's OK can be ticked here")
	}
	if err := h.choreRepo.SetChorePendingApproval(ctx, chore, nil, performer, &at); err != nil {
		return nil, fmt.Errorf("submitting: %w", err)
	}
	updated, err := h.choreRepo.GetChore(ctx, chore.ID, actor.ID, chore.CircleID)
	if err != nil {
		return nil, fmt.Errorf("reloading chore: %w", err)
	}
	if h.realTimeService != nil {
		changes := map[string]interface{}{
			"status":    chModel.ChoreStatusPendingApproval,
			"updatedBy": performer,
			"updatedAt": time.Now().UTC(),
		}
		h.realTimeService.GetEventBroadcaster().BroadcastChoreUpdated(updated, &actor.User, changes, nil)
	}
	return updated, nil
}
