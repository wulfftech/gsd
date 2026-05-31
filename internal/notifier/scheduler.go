package notifier

import (
	"context"
	"log"
	"time"

	"donetick.com/core/config"
	chRepo "donetick.com/core/internal/chore/repo"
	chSched "donetick.com/core/internal/chore/scheduling"
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
}

func NewScheduler(cfg *config.Config, ur *uRepo.UserRepository, cr *chRepo.ChoreRepository, n *Notifier, nr *nRepo.NotificationRepository, ep *events.EventsProducer) *Scheduler {
	return &Scheduler{
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
// missOverdueChoresJob finds all active recurring chores whose due date has
// passed, marks each missed period as ChoreHistoryStatusMissed, and advances
// NextDueDate to the next future occurrence.  This keeps the chore list clean
// and builds an accurate history of missed tasks.
func (s *Scheduler) missOverdueChoresJob(c context.Context) (time.Duration, error) {
	log := logging.FromContext(c)
	startTime := time.Now().UTC()

	chores, err := s.choreRepo.GetOverdueRecurringChores(c)
	if err != nil {
		log.Error("missOverdueChoresJob: error fetching overdue chores", "err", err)
		return time.Since(startTime), err
	}

	log.Debug("missOverdueChoresJob: processing overdue chores", "count", len(chores))

	const maxMissedPerChore = 30 // safety cap to avoid hammering the DB for long-neglected chores

	for _, chore := range chores {
		for i := 0; i < maxMissedPerChore; i++ {
			if chore.NextDueDate == nil || !chore.NextDueDate.Before(time.Now().UTC()) {
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

			if err := s.choreRepo.MarkChoreAsMissed(c, chore, nextDueDate); err != nil {
				log.Error("missOverdueChoresJob: error marking chore as missed",
					"choreID", chore.ID, "err", err)
				break
			}

			// Update the local copy so the next loop iteration uses the new due date.
			chore.NextDueDate = nextDueDate
		}
	}

	return time.Since(startTime), nil
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
