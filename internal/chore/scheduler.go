package chore

import (
	"context"
	"math/rand"
	"time"

	chModel "donetick.com/core/internal/chore/model"
	chSched "donetick.com/core/internal/chore/scheduling"
)

// scheduleNextDueDate delegates to the exported scheduling package so that
// both this package and the notifier package can share the logic without a
// circular import.
func scheduleNextDueDate(ctx context.Context, chore *chModel.Chore, completedDate time.Time) (*time.Time, error) {
	return chSched.ScheduleNextDueDate(ctx, chore, completedDate)
}

func scheduleAdaptiveNextDueDate(chore *chModel.Chore, completedDate time.Time, history []*chModel.ChoreHistory) (*time.Time, error) {
	return chSched.ScheduleAdaptiveNextDueDate(chore, completedDate, history)
}

func RemoveAssigneeAndReassign(chore *chModel.Chore, userID int) {
	for i, assignee := range chore.Assignees {
		if assignee.UserID == userID {
			chore.Assignees = append(chore.Assignees[:i], chore.Assignees[i+1:]...)
			break
		}
	}

	// Handle no assignee strategy
	switch {
	case chore.AssignStrategy == chModel.AssignmentStrategyNoAssignee:
		chore.AssignedTo = nil // Set to nil to indicate no assignee
	case len(chore.Assignees) == 0:
		createdBy := chore.CreatedBy
		chore.AssignedTo = &createdBy
	default:
		userID := chore.Assignees[rand.Intn(len(chore.Assignees))].UserID
		chore.AssignedTo = &userID
	}
	chore.UpdatedAt = time.Now().UTC()
}
