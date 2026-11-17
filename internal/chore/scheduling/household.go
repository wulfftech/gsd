package scheduling

import (
	"context"
	"time"

	chModel "donetick.com/core/internal/chore/model"
)

// HolidayRange is a [Start, End) span of local time when school is out.
type HolidayRange struct {
	Start time.Time
	End   time.Time
}

// StartOfDay is local midnight of t's day in loc.
func StartOfDay(t time.Time, loc *time.Location) time.Time {
	l := t.In(loc)
	return time.Date(l.Year(), l.Month(), l.Day(), 0, 0, 0, 0, loc)
}

// InHoliday reports whether t falls inside any of the ranges.
func InHoliday(t time.Time, holidays []HolidayRange) bool {
	for _, h := range holidays {
		if !t.Before(h.Start) && t.Before(h.End) {
			return true
		}
	}
	return false
}

// FinaliseNextDueDate applies the household rules to a next due date that the
// frequency rules produced: it never lands on a day before today, and for a
// chore that skips school holidays it never lands inside one. It steps forward
// one occurrence at a time using the chore's own frequency, so a weekday job
// moves to the next weekday after the holiday, not to an arbitrary date.
func FinaliseNextDueDate(ctx context.Context, chore *chModel.Chore, next *time.Time, now time.Time, loc *time.Location, holidays []HolidayRange) (*time.Time, error) {
	if next == nil {
		return nil, nil
	}
	today := StartOfDay(now, loc)
	cur := *next
	const maxSteps = 1000 // a year of daily steps plus slack; stops a bad rule spinning forever
	for i := 0; i < maxSteps; i++ {
		inPast := cur.Before(today)
		inHoliday := chore.SkipHolidays && InHoliday(cur, holidays)
		if !inPast && !inHoliday {
			out := cur.UTC()
			return &out, nil
		}
		step := *chore
		step.IsRolling = false
		stepDue := cur
		step.NextDueDate = &stepDue
		n, err := ScheduleNextDueDate(ctx, &step, cur)
		if err != nil {
			return nil, err
		}
		if n == nil || !n.After(cur) {
			// adaptive and other self-referential rules can stand still; fall forward a day
			t := cur.AddDate(0, 0, 1)
			n = &t
		}
		cur = *n
	}
	out := cur.UTC()
	return &out, nil
}

// mondayOf is local midnight of the Monday starting t's week.
func mondayOf(t time.Time, loc *time.Location) time.Time {
	d := StartOfDay(t, loc)
	offset := (int(d.Weekday()) + 6) % 7 // Monday=0 ... Sunday=6
	return d.AddDate(0, 0, -offset)
}

// RotationOrder is the explicit order if set, else the assignee order.
func RotationOrder(chore *chModel.Chore) []int {
	if chore.RotationOrder != nil && len(*chore.RotationOrder) > 0 {
		return []int(*chore.RotationOrder)
	}
	ids := make([]int, 0, len(chore.Assignees))
	for _, a := range chore.Assignees {
		ids = append(ids, a.UserID)
	}
	return ids
}

// RotationAssignee is who holds a weekly_rotation chore in the week containing
// due. Weeks start on Monday in loc. Nil when the chore has no one to rotate.
func RotationAssignee(chore *chModel.Chore, due time.Time, loc *time.Location) *int {
	order := RotationOrder(chore)
	if len(order) == 0 {
		return nil
	}
	anchor := chore.CreatedAt
	if chore.RotationAnchor != nil {
		anchor = *chore.RotationAnchor
	}
	a := mondayOf(anchor, loc)
	d := mondayOf(due, loc)
	// Round, don't truncate: a week is not always 168 hours across a DST change.
	weeks := int((d.Sub(a).Hours() + 12) / (24 * 7))
	if d.Before(a) {
		weeks = -int((a.Sub(d).Hours() + 12) / (24 * 7))
	}
	idx := ((weeks % len(order)) + len(order)) % len(order)
	id := order[idx]
	return &id
}
