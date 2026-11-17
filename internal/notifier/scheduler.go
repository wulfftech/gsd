package notifier

import (
	"context"
	"log"
	"time"

	"donetick.com/core/config"
	chModel "donetick.com/core/internal/chore/model"
	chRepo "donetick.com/core/internal/chore/repo"
	chSched "donetick.com/core/internal/chore/scheduling"
	"donetick.com/core/internal/household"
	"donetick.com/core/internal/events"
	nRepo "donetick.com/core/internal/notifier/repo"
	uRepo "donetick.com/core/internal/user/repo"
	"donetick.com/core/logging"
)

type keyType string

const (
	SchedulerKey keyType = "scheduler"
)

type Scheduler struct {
	choreRepo        *chRepo.ChoreRepository
	userRepo         *uRepo.UserRepository
	stopChan         chan bool
	notifier         *Notifier
	eventsProducer   *events.EventsProducer
	notificationRepo *nRepo.NotificationRepository
	SchedulerJobs    config.SchedulerConfig
	householdRepo    *household.Repository
}

func NewScheduler(cfg *config.Config, ur *uRepo.UserRepository, cr *chRepo.ChoreRepository, n *Notifier, nr *nRepo.NotificationRepository, ep *events.EventsProducer, hr *household.Repository) *Scheduler {
	return &Scheduler{
		householdRepo:    hr,
		choreRepo:        cr,
		userRepo:         ur,
		stopChan:         make(chan bool),
		notifier:         n,
		notificationRepo: nr,
		eventsProducer:   ep,
		SchedulerJobs:    cfg.SchedulerJobs,
	}
}

func (s *Scheduler) Start(c context.Context) {
	log := logging.FromContext(c)
	log.Debug("Scheduler started")
	go s.runScheduler(c, " NOTIFICATION_SCHEDULER ", s.loadAndSendNotificationJob, 3*time.Minute)
	go s.runScheduler(c, " NOTIFICATION_CLEANUP ", s.cleanupSentNotifications, 24*time.Hour*30)
	go s.runScheduler(c, " MISSED_CHORES ", s.missOverdueChoresJob, 1*time.Hour)
}
func (s *Scheduler) cleanupSentNotifications(c context.Context) (time.Duration, error) {
	log := logging.FromContext(c)
	deleteBefore := time.Now().UTC().Add(-time.Hour * 24 * 30)
	err := s.notificationRepo.DeleteSentNotifications(c, deleteBefore)
	if err != nil {
		log.Error("Error deleting sent notifications", err)
		return time.Duration(0), err
	}
	return time.Duration(0), nil
}

func (s *Scheduler) loadAndSendNotificationJob(c context.Context) (time.Duration, error) {
	log := logging.FromContext(c)
	startTime := time.Now().UTC()
	getAllPendingNotifications, err := s.notificationRepo.GetPendingNotification(c, time.Minute*900)
	log.Debug("Getting pending notifications", " count ", len(getAllPendingNotifications))

	if err != nil {
		log.Error("Error getting pending notifications")
		return time.Since(startTime), err
	}

	for _, notification := range getAllPendingNotifications {
		err := s.notifier.SendNotification(c, notification)
		if err != nil {
			log.Error("Error sending notification", err)
			continue
		}
		if notification.RawEvent != nil && notification.WebhookURL != nil {
			// if we have a webhook url, we should send the event to the webhook
			s.eventsProducer.NotificationEvent(c, *notification.WebhookURL, notification.RawEvent)
		}

		notification.IsSent = true
	}

	s.notificationRepo.MarkNotificationsAsSent(getAllPendingNotifications)
	return time.Since(startTime), nil
}

// missOverdueChoresJob logs a missed history entry for each recurring chore
// occurrence that wasn't done by the end of its day, and moves the chore on to
// its next occurrence. A chore stays on today's list until local midnight, so a
// job not done by its time can still be done late the same day. It also keeps
// weekly-rotation assignees in step with the calendar.
func (s *Scheduler) missOverdueChoresJob(c context.Context) (time.Duration, error) {
	log := logging.FromContext(c)
	startTime := time.Now().UTC()
	loc := time.Local
	today := chSched.StartOfDay(time.Now().UTC(), loc)

	chores, err := s.choreRepo.GetOverdueRecurringChores(c, today)
	if err != nil {
		log.Error("missOverdueChoresJob: error fetching overdue chores", "err", err)
		return time.Since(startTime), err
	}

	log.Debug("missOverdueChoresJob: processing overdue chores", "count", len(chores))

	const maxMissedPerChore = 30 // safety cap to avoid hammering the DB for long-neglected chores
	holidays := map[int][]chSched.HolidayRange{}

	for _, chore := range chores {
		hs, ok := holidays[chore.CircleID]
		if !ok {
			hs = s.holidayRanges(c, chore.CircleID, loc)
			holidays[chore.CircleID] = hs
		}
		for i := 0; i < maxMissedPerChore; i++ {
			if chore.NextDueDate == nil || !chore.NextDueDate.Before(today) {
				break
			}

			nextDueDate, err := chSched.ScheduleNextDueDate(c, chore, chore.NextDueDate.UTC())
			if err != nil {
				log.Error("missOverdueChoresJob: error computing next due date",
					"choreID", chore.ID, "err", err)
				break
			}

			// Guard against frequency types that don't advance (e.g. adaptive when
			// completedDate == nextDueDate gives diff=0).  Fall forward by 1 day.
			if nextDueDate == nil || !nextDueDate.After(*chore.NextDueDate) {
				tomorrow := chore.NextDueDate.Add(24 * time.Hour)
				nextDueDate = &tomorrow
			}
			// Skip holidays for chores that ask to. The past is left alone here:
			// each missed day before today gets its own history row.
			if chore.SkipHolidays {
				nextDueDate, err = chSched.FinaliseNextDueDate(c, chore, nextDueDate, time.Time{}, loc, hs)
				if err != nil || nextDueDate == nil {
					log.Error("missOverdueChoresJob: error skipping holidays", "choreID", chore.ID, "err", err)
					break
				}
			}

			if err := s.choreRepo.MarkChoreAsMissed(c, chore, nextDueDate); err != nil {
				log.Error("missOverdueChoresJob: error marking chore as missed",
					"choreID", chore.ID, "err", err)
				break
			}

			// Update the local copy so the next loop iteration uses the new due date.
			chore.NextDueDate = nextDueDate
		}
	}

	s.syncRotations(c, loc)
	return time.Since(startTime), nil
}

func (s *Scheduler) holidayRanges(c context.Context, circleID int, loc *time.Location) []chSched.HolidayRange {
	if s.householdRepo == nil {
		return nil
	}
	rs, err := s.householdRepo.HolidayRanges(c, circleID, loc)
	if err != nil {
		logging.FromContext(c).Error("missOverdueChoresJob: error loading holidays", "circleID", circleID, "err", err)
		return nil
	}
	out := make([]chSched.HolidayRange, len(rs))
	for i, r := range rs {
		out[i] = chSched.HolidayRange{Start: r.Start, End: r.End}
	}
	return out
}

// syncRotations sets each weekly-rotation chore's assignee from its due date,
// so the role moves on each Monday even when nobody completed anything.
// A chore waiting for approval keeps the kid who did it.
func (s *Scheduler) syncRotations(c context.Context, loc *time.Location) {
	log := logging.FromContext(c)
	chores, err := s.choreRepo.GetRotationChores(c, 0)
	if err != nil {
		log.Error("syncRotations: error fetching rotation chores", "err", err)
		return
	}
	for _, ch := range chores {
		if ch.NextDueDate == nil || ch.Status == chModel.ChoreStatusPendingApproval {
			continue
		}
		want := chSched.RotationAssignee(ch, *ch.NextDueDate, loc)
		if want == nil || (ch.AssignedTo != nil && *ch.AssignedTo == *want) {
			continue
		}
		if err := s.choreRepo.SetAssignedTo(c, ch.ID, want); err != nil {
			log.Error("syncRotations: error reassigning", "choreID", ch.ID, "err", err)
		}
	}
}

func (s *Scheduler) runScheduler(c context.Context, jobName string, job func(c context.Context) (time.Duration, error), interval time.Duration) {

	for {
		logging.FromContext(c).Debug("Scheduler running ", jobName, " time", time.Now().UTC().String())

		select {
		case <-s.stopChan:
			log.Println("Scheduler stopped")
			return
		default:
			elapsedTime, err := job(c)
			if err != nil {
				logging.FromContext(c).Error("Error running scheduler job", err)
			}
			logging.FromContext(c).Debug("Scheduler job completed", jobName, " time: ", elapsedTime.String())
		}
		time.Sleep(interval)
	}
}

func (s *Scheduler) Stop() {
	s.stopChan <- true
}
