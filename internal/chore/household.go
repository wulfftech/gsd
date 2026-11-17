package chore

import (
	"context"
	"fmt"
	"time"

	auth "donetick.com/core/internal/auth"
	chModel "donetick.com/core/internal/chore/model"
	chSched "donetick.com/core/internal/chore/scheduling"
	circle "donetick.com/core/internal/circle/model"
	"donetick.com/core/internal/household"
	uModel "donetick.com/core/internal/user/model"
	"donetick.com/core/logging"
	"github.com/gin-gonic/gin"
)

// localZone is the household's timezone: the server's. "Today", "midnight" and
// "Monday" all mean local time, never UTC.
var localZone = func() *time.Location { return time.Local }

// nowFunc is swapped in tests.
var nowFunc = time.Now

// HolidayRanges converts stored holidays into scheduling ranges.
func HolidayRanges(ctx context.Context, repo *household.Repository, circleID int) []chSched.HolidayRange {
	if repo == nil {
		return nil
	}
	rs, err := repo.HolidayRanges(ctx, circleID, localZone())
	if err != nil {
		logging.FromContext(ctx).Errorw("failed to load school holidays; scheduling without them", "error", err, "circleID", circleID)
		return nil
	}
	out := make([]chSched.HolidayRange, len(rs))
	for i, r := range rs {
		out[i] = chSched.HolidayRange{Start: r.Start, End: r.End}
	}
	return out
}

// finaliseDue applies the household scheduling rules to a computed due date.
func (h *Handler) finaliseDue(ctx context.Context, chore *chModel.Chore, next *time.Time) (*time.Time, error) {
	return chSched.FinaliseNextDueDate(ctx, chore, next, nowFunc(), localZone(), HolidayRanges(ctx, h.hRepo, chore.CircleID))
}

// planNext works out the next due date and next assignee after chore was
// performed at performedAt by performer. It is the single place completion,
// approval and the eapi share, so the household rules can't drift between them.
func (h *Handler) planNext(ctx context.Context, chore *chModel.Chore, performedAt time.Time, performer int, history []*chModel.ChoreHistory) (*time.Time, *int, error) {
	var next *time.Time
	var err error
	if chore.FrequencyType == chModel.FrequencyTypeAdaptive {
		limited := history
		if len(limited) > 5 {
			limited = limited[:5]
		}
		next, err = scheduleAdaptiveNextDueDate(chore, performedAt, limited)
	} else {
		next, err = scheduleNextDueDate(ctx, chore, performedAt.UTC())
	}
	if err != nil {
		return nil, nil, fmt.Errorf("scheduling next due date: %w", err)
	}
	next, err = h.finaliseDue(ctx, chore, next)
	if err != nil {
		return nil, nil, fmt.Errorf("applying household rules: %w", err)
	}
	assignee, err := checkNextAssignee(chore, history, performer)
	if err != nil {
		return nil, nil, fmt.Errorf("choosing next assignee: %w", err)
	}
	if chore.AssignStrategy == chModel.AssignmentStrategyWeeklyRotation && next != nil {
		assignee = chSched.RotationAssignee(chore, *next, localZone())
	}
	return next, assignee, nil
}

// applyHouseholdFields copies and validates the household fields of a request.
func applyHouseholdFields(chore *chModel.Chore, req *chModel.ChoreReq) error {
	if !req.Checkpoint.Valid() {
		return fmt.Errorf("checkpoint must be one of morning, afternoon, evening, extra or empty")
	}
	chore.Checkpoint = req.Checkpoint
	chore.SkipHolidays = req.SkipHolidays
	chore.RotationOrder = req.RotationOrder
	chore.RotationAnchor = req.RotationAnchor
	if req.AssignStrategy != chModel.AssignmentStrategyWeeklyRotation {
		return nil
	}
	assignees := map[int]bool{}
	for _, a := range req.Assignees {
		assignees[a.UserID] = true
	}
	if req.RotationOrder != nil {
		seen := map[int]bool{}
		for _, id := range *req.RotationOrder {
			if !assignees[id] {
				return fmt.Errorf("everyone in the rotation must also be an assignee")
			}
			if seen[id] {
				return fmt.Errorf("someone is in the rotation twice")
			}
			seen[id] = true
		}
	}
	if len(chSched.RotationOrder(rotationView(chore, req))) == 0 {
		return fmt.Errorf("weekly rotation needs at least one person")
	}
	return nil
}

// rotationView is the chore as the rotation sees it: with the request's assignees.
func rotationView(chore *chModel.Chore, req *chModel.ChoreReq) *chModel.Chore {
	c := *chore
	c.Assignees = nil
	for _, a := range req.Assignees {
		c.Assignees = append(c.Assignees, chModel.ChoreAssignees{UserID: a.UserID})
	}
	return &c
}

// prepareHouseholdChore runs on create and edit: validates the household
// fields, moves a due date out of school holidays for chores that skip them,
// and sets the assignee of a weekly rotation from the due date.
func (h *Handler) prepareHouseholdChore(ctx context.Context, chore *chModel.Chore, req *chModel.ChoreReq) error {
	if err := applyHouseholdFields(chore, req); err != nil {
		return err
	}
	if chore.NextDueDate != nil && chore.SkipHolidays {
		// time.Time{} as "now": only the holiday rule applies here. A parent who
		// deliberately sets a past due date keeps it.
		due, err := chSched.FinaliseNextDueDate(ctx, chore, chore.NextDueDate, time.Time{}, localZone(), HolidayRanges(ctx, h.hRepo, chore.CircleID))
		if err != nil {
			return err
		}
		chore.NextDueDate = due
	}
	if chore.AssignStrategy == chModel.AssignmentStrategyWeeklyRotation && chore.NextDueDate != nil {
		chore.AssignedTo = chSched.RotationAssignee(rotationView(chore, req), *chore.NextDueDate, localZone())
	}
	return nil
}

func circleRole(circleUsers []*circle.UserCircleDetail, userID int) circle.UserRole {
	for _, cu := range circleUsers {
		if cu.UserID == userID {
			return cu.Role
		}
	}
	return ""
}

// isManager is true for admins and managers of the circle.
func (h *Handler) isManager(ctx context.Context, user *uModel.UserDetails) (bool, error) {
	users, err := h.circleRepo.GetCircleUsers(ctx, user.CircleID)
	if err != nil {
		return false, err
	}
	r := circleRole(users, user.ID)
	return r == circle.UserRoleAdmin || r == circle.UserRoleManager, nil
}

// requireManager writes a 403 and returns false when the caller is a plain
// member. Members (the kids) can view, complete and undo, and nothing else.
func (h *Handler) requireManager(c *gin.Context) bool {
	user, ok := auth.CurrentUser(c)
	if !ok {
		c.JSON(401, gin.H{"error": "Authentication failed"})
		return false
	}
	ok, err := h.isManager(c, user)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to retrieve circle users"})
		return false
	}
	if !ok {
		c.JSON(403, gin.H{"error": "Only a parent can do that"})
		return false
	}
	return true
}

// managerOnly is middleware form of requireManager.
func (h *Handler) managerOnly(c *gin.Context) {
	if !h.requireManager(c) {
		c.Abort()
		return
	}
	c.Next()
}
