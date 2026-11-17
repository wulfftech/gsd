package scheduling

import (
	"context"
	"testing"
	"time"

	chModel "donetick.com/core/internal/chore/model"
)

func perth(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Australia/Perth")
	if err != nil {
		t.Skip("no tzdata:", err)
	}
	return loc
}

func at(loc *time.Location, y int, m time.Month, d, hh, mm int) time.Time {
	return time.Date(y, m, d, hh, mm, 0, 0, loc)
}

func dailyChore(due time.Time) *chModel.Chore {
	d := due.UTC()
	return &chModel.Chore{ID: 1, FrequencyType: chModel.FrequencyTypeDaily, Frequency: 1, NextDueDate: &d,
		FrequencyMetadataV2: &chModel.FrequencyMetadata{}}
}

func TestFinaliseNeverSchedulesIntoAPastDay(t *testing.T) {
	loc := perth(t)
	// Ticked Monday 07:10, approved Wednesday 08:00: the next occurrence the
	// frequency rule gives (Tuesday 07:00) is a past day, so it moves to
	// Wednesday 07:00, which is today and still due.
	mon := at(loc, 2026, 11, 23, 7, 0)
	ch := dailyChore(mon)
	next, err := ScheduleNextDueDate(context.Background(), ch, at(loc, 2026, 11, 23, 7, 10).UTC())
	if err != nil {
		t.Fatal(err)
	}
	now := at(loc, 2026, 11, 25, 8, 0)
	got, err := FinaliseNextDueDate(context.Background(), ch, next, now, loc, nil)
	if err != nil {
		t.Fatal(err)
	}
	if want := at(loc, 2026, 11, 25, 7, 0); !got.Equal(want) {
		t.Fatalf("got %v, want %v", got.In(loc), want)
	}
}

func TestFinaliseSameDayApprovalGoesToTomorrow(t *testing.T) {
	loc := perth(t)
	ch := dailyChore(at(loc, 2026, 11, 23, 7, 0))
	next, _ := ScheduleNextDueDate(context.Background(), ch, at(loc, 2026, 11, 23, 7, 10).UTC())
	got, err := FinaliseNextDueDate(context.Background(), ch, next, at(loc, 2026, 11, 23, 20, 0), loc, nil)
	if err != nil {
		t.Fatal(err)
	}
	if want := at(loc, 2026, 11, 24, 7, 0); !got.Equal(want) {
		t.Fatalf("got %v, want %v", got.In(loc), want)
	}
}

func TestFinaliseSkipsHolidaysForWeekdayChore(t *testing.T) {
	loc := perth(t)
	days := []*string{}
	for _, d := range []string{"monday", "tuesday", "wednesday", "thursday", "friday"} {
		d := d
		days = append(days, &d)
	}
	due := at(loc, 2026, 12, 17, 16, 0).UTC() // Thursday, last day of term
	ch := &chModel.Chore{ID: 2, FrequencyType: chModel.FrequencyTypeDayOfTheWeek, NextDueDate: &due, SkipHolidays: true,
		FrequencyMetadataV2: &chModel.FrequencyMetadata{Days: days, Time: at(loc, 2026, 5, 10, 16, 0).Format(time.RFC3339), Timezone: "Australia/Perth"}}
	holidays := []HolidayRange{{Start: at(loc, 2026, 12, 18, 0, 0), End: at(loc, 2027, 2, 1, 0, 0)}}
	next, err := ScheduleNextDueDate(context.Background(), ch, due)
	if err != nil {
		t.Fatal(err)
	}
	got, err := FinaliseNextDueDate(context.Background(), ch, next, at(loc, 2026, 12, 17, 17, 0), loc, holidays)
	if err != nil {
		t.Fatal(err)
	}
	if want := at(loc, 2027, 2, 1, 16, 0); !got.Equal(want) {
		t.Fatalf("got %v, want %v (first school day after the holidays)", got.In(loc), want)
	}

	ch.SkipHolidays = false
	got, _ = FinaliseNextDueDate(context.Background(), ch, next, at(loc, 2026, 12, 17, 17, 0), loc, holidays)
	if want := at(loc, 2026, 12, 18, 16, 0); !got.Equal(want) {
		t.Fatalf("without skipHolidays got %v, want %v", got.In(loc), want)
	}
}

func TestRotationMatchesThePlan(t *testing.T) {
	loc := perth(t)
	const ann, bob, cat = 2, 3, 4
	anchor := at(loc, 2026, 11, 23, 0, 0) // Monday of week A
	mk := func(order ...int) *chModel.Chore {
		o := chModel.IntList(order)
		return &chModel.Chore{RotationOrder: &o, RotationAnchor: &anchor}
	}
	// Three roles over three people: each week every role moves to the next person.
	dishes, table, tidy := mk(ann, cat, bob), mk(bob, ann, cat), mk(cat, bob, ann)
	weeks := []struct {
		day               time.Time
		dishes, table, td int
	}{
		{at(loc, 2026, 11, 23, 7, 0), ann, bob, cat},  // Monday week A
		{at(loc, 2026, 11, 29, 21, 0), ann, bob, cat}, // Sunday night still week A
		{at(loc, 2026, 11, 30, 7, 0), cat, ann, bob},  // week B
		{at(loc, 2026, 12, 7, 7, 0), bob, cat, ann},   // week C
		{at(loc, 2026, 12, 14, 7, 0), ann, bob, cat},  // back to A
		{at(loc, 2026, 11, 16, 7, 0), bob, cat, ann},  // the week before the anchor is C
	}
	for _, w := range weeks {
		if g := *RotationAssignee(dishes, w.day, loc); g != w.dishes {
			t.Errorf("%v dishes: got %d want %d", w.day, g, w.dishes)
		}
		if g := *RotationAssignee(table, w.day, loc); g != w.table {
			t.Errorf("%v table: got %d want %d", w.day, g, w.table)
		}
		if g := *RotationAssignee(tidy, w.day, loc); g != w.td {
			t.Errorf("%v tidy: got %d want %d", w.day, g, w.td)
		}
	}
	if RotationAssignee(&chModel.Chore{}, anchor, loc) != nil {
		t.Error("empty rotation should have no assignee")
	}
}
