package chore

import (
	"fmt"
	"strconv"
	"time"

	"donetick.com/core/config"
	"donetick.com/core/internal/auth"
	authMiddleware "donetick.com/core/internal/auth"
	chRepo "donetick.com/core/internal/chore/repo"
	chSched "donetick.com/core/internal/chore/scheduling"
	"donetick.com/core/internal/events"
	nps "donetick.com/core/internal/notifier/service"
	"donetick.com/core/internal/utils"
	"donetick.com/core/logging"
	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"

	limiter "github.com/ulule/limiter/v3"

	chModel "donetick.com/core/internal/chore/model"
	cRepo "donetick.com/core/internal/circle/repo"
	stRepo "donetick.com/core/internal/subtask/repo"
	uRepo "donetick.com/core/internal/user/repo"
)

type API struct {
	hh            *Handler
	choreRepo     *chRepo.ChoreRepository
	userRepo      *uRepo.UserRepository
	circleRepo    *cRepo.CircleRepository
	nPlanner      *nps.NotificationPlanner
	eventProducer *events.EventsProducer
	stRepo        *stRepo.SubTasksRepository
}

func NewAPI(cr *chRepo.ChoreRepository, userRepo *uRepo.UserRepository, circleRepo *cRepo.CircleRepository, nPlanner *nps.NotificationPlanner, eventProducer *events.EventsProducer, stRepo *stRepo.SubTasksRepository, hh *Handler) *API {
	return &API{
		hh:            hh,
		choreRepo:     cr,
		userRepo:      userRepo,
		circleRepo:    circleRepo,
		nPlanner:      nPlanner,
		eventProducer: eventProducer,
		stRepo:        stRepo,
	}
}

func (h *API) GetAllChores(c *gin.Context) {
	user := auth.MustCurrentUser(c)
	chores, err := h.choreRepo.GetChores(c, user.CircleID, user.ID, false)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, chores)
}

// GetChore returns a single chore by ID, scoped to the caller's circle. Reuses
// the same choreRepo.GetChore(id, userID, circleID) call the rest of this file
// already uses — that query filters on chores.circle_id = ? at the SQL level, so
// a chore belonging to a different circle simply doesn't match and GORM returns
// gorm.ErrRecordNotFound, which we surface as a plain 404 (not "chore exists but
// isn't yours") to avoid leaking cross-circle existence.
//
// Response shape is intentionally unwrapped (the bare chore object), matching
// GetAllChores above (c.JSON(200, chores) — a raw array of the same objects),
// so a client that filters the eapi list client-side today gets an identical
// single object from this endpoint.
func (h *API) GetChore(c *gin.Context) {
	log := logging.FromContext(c)
	choreIDRaw := c.Param("id")
	choreID, err := strconv.Atoi(choreIDRaw)
	if err != nil {
		log.Debugw("chore.api.GetChore failed to parse chore ID", "error", err)
		c.JSON(400, gin.H{"error": "Invalid chore ID"})
		return
	}

	user := auth.MustCurrentUser(c)
	chore, err := h.choreRepo.GetChore(c, choreID, user.ID, user.CircleID)
	if err != nil {
		c.JSON(404, gin.H{"error": "Chore not found"})
		return
	}
	c.JSON(200, chore)
}

func (h *API) CreateChore(c *gin.Context) {
	log := logging.FromContext(c)
	var choreRequest chModel.ChoreLiteReq
	user := auth.MustCurrentUser(c)

	if err := c.BindJSON(&choreRequest); err != nil {
		log.Debugw("chore.api.CreateChore failed to bind JSON", "error", err)
		c.JSON(400, gin.H{"error": "Invalid request body"})
		return
	}

	// Validate required fields
	if choreRequest.Name == "" {
		c.JSON(400, gin.H{"error": "Chore name is required"})
		return
	}

	// Parse due date if provided
	var nextDueDate *time.Time
	if choreRequest.DueDate != "" {
		parsedDate, err := time.Parse(time.RFC3339, choreRequest.DueDate)
		if err != nil {
			parsedDateSimple, errSimple := time.Parse("2006-01-02", choreRequest.DueDate)
			if errSimple != nil {
				c.JSON(400, gin.H{"error": "Invalid due date format. Use RFC3339 or YYYY-MM-DD"})
				return
			}
			// Set time to now UTC
			now := time.Now().UTC()
			parsedDate = time.Date(parsedDateSimple.Year(), parsedDateSimple.Month(), parsedDateSimple.Day(), now.Hour(), now.Minute(), now.Second(), 0, time.UTC)
			err = nil
		}
		if err != nil {
			c.JSON(400, gin.H{"error": "Invalid due date format. Use RFC3339 format"})
			return
		}
		nextDueDate = &parsedDate
	}
	// get all circle members:
	circleUsers, err := h.circleRepo.GetCircleUsers(c, user.CircleID)
	if err != nil {
		log.Errorw("chore.api.CreateChore failed to get circle users", "error", err)
		c.JSON(500, gin.H{"error": "Failed to get circle members"})
		return
	}
	createdBy := user.ID
	if choreRequest.CreatedBy != nil {
		// Check if the specified user exists in the circle
		var found bool
		for _, u := range circleUsers {
			if u.UserID == *choreRequest.CreatedBy {
				found = true
				createdBy = u.UserID
				break
			}
		}
		if !found {
			log.Errorw("chore.api.CreateChore specified user not found in circle", "userID", *choreRequest.CreatedBy)
			c.JSON(400, gin.H{"error": "Specified user not found in circle"})
			return
		}
	}

	// FrequencyType: default to "once" (previous hardcoded behavior) so existing
	// callers that never send this field are unaffected. Only the frequency types
	// that need no extra scheduling metadata are accepted here -- "interval",
	// "days_of_the_week", "day_of_the_month" (need FrequencyMetadataV2: unit/days/
	// time/timezone), "adaptive" (needs completion history) and "trigger" (needs a
	// ThingTrigger association) all require inputs this lite eapi payload doesn't
	// carry, so rather than half-support them (and risk a chore that silently never
	// reschedules, per scheduling.ScheduleNextDueDate) we reject them outright with
	// a clear 400. Callers that need those can use the full web /api/v1/chores path.
	frequencyType := chModel.FrequencyTypeOnce
	if choreRequest.FrequencyType != nil && *choreRequest.FrequencyType != "" {
		requested := chModel.FrequencyType(*choreRequest.FrequencyType)
		switch requested {
		case chModel.FrequencyTypeOnce, chModel.FrequencyTypeDaily, chModel.FrequencyTypeWeekly,
			chModel.FrequencyTypeMonthly, chModel.FrequencyTypeYearly, chModel.FrequencyTypeNoRepeat:
			frequencyType = requested
		case chModel.FrequencyTypeInterval, chModel.FrequencyTypeDayOfTheWeek, chModel.FrequencyTypeDayOfTheMonth,
			chModel.FrequencyTypeAdaptive, chModel.FrequencyTypeTrigger:
			log.Debugw("chore.api.CreateChore unsupported frequencyType requiring metadata", "frequencyType", requested)
			c.JSON(400, gin.H{"error": fmt.Sprintf(
				"frequencyType %q requires additional metadata not supported by this endpoint; supported values are: once, daily, weekly, monthly, yearly, no_repeat",
				*choreRequest.FrequencyType)})
			return
		default:
			log.Debugw("chore.api.CreateChore invalid frequencyType", "frequencyType", *choreRequest.FrequencyType)
			c.JSON(400, gin.H{"error": fmt.Sprintf(
				"invalid frequencyType %q; supported values are: once, daily, weekly, monthly, yearly, no_repeat",
				*choreRequest.FrequencyType)})
			return
		}
	}

	// AssignStrategy: default to "random" (previous hardcoded behavior). Unlike
	// FrequencyType, every strategy value is self-contained (it only governs how
	// the *next* assignee is picked on completion -- see checkNextAssignee) so all
	// of them are accepted, validated against the same enum handler.go's web path
	// uses (chModel.AssignmentStrategy*), never an arbitrary string.
	assignStrategy := chModel.AssignmentStrategyRandom
	if choreRequest.AssignStrategy != nil && *choreRequest.AssignStrategy != "" {
		requested := chModel.AssignmentStrategy(*choreRequest.AssignStrategy)
		switch requested {
		case chModel.AssignmentStrategyRandom, chModel.AssignmentStrategyLeastAssigned, chModel.AssignmentStrategyLeastCompleted,
			chModel.AssignmentStrategyKeepLastAssigned, chModel.AssignmentStrategyRandomExceptLastAssigned,
			chModel.AssignmentStrategyRoundRobin, chModel.AssignmentStrategyNoAssignee:
			assignStrategy = requested
		default:
			log.Debugw("chore.api.CreateChore invalid assignStrategy", "assignStrategy", *choreRequest.AssignStrategy)
			c.JSON(400, gin.H{"error": fmt.Sprintf(
				"invalid assignStrategy %q; supported values are: random, least_assigned, least_completed, keep_last_assigned, random_except_last_assigned, round_robin, no_assignee",
				*choreRequest.AssignStrategy)})
			return
		}
	}

	// AssignedTo: default to the creator (previous hardcoded behavior). A caller
	// may instead name a specific circle member -- validated against circleUsers
	// the same way handler.go's web path validates assignees ("Assignee not found
	// in circle"). "no_assignee" strategy is mutually exclusive with an explicit
	// assignedTo (mirrors Chore.CanComplete/RemoveAssigneeAndReassign, which treat
	// AssignedTo == nil as the no-assignee signal), so combining them is a 400
	// rather than silently picking one.
	var assignedTo *int
	var assignees []chModel.ChoreAssignees
	if assignStrategy == chModel.AssignmentStrategyNoAssignee {
		if choreRequest.AssignedTo != nil {
			c.JSON(400, gin.H{"error": "assignedTo cannot be set when assignStrategy is no_assignee"})
			return
		}
	} else {
		assigneeID := createdBy
		if choreRequest.AssignedTo != nil {
			var found bool
			for _, u := range circleUsers {
				if u.UserID == *choreRequest.AssignedTo {
					found = true
					break
				}
			}
			if !found {
				log.Errorw("chore.api.CreateChore assignedTo user not found in circle", "userID", *choreRequest.AssignedTo)
				c.JSON(400, gin.H{"error": "Assignee not found in circle"})
				return
			}
			assigneeID = *choreRequest.AssignedTo
		}
		assignedTo = &assigneeID
		assignees = []chModel.ChoreAssignees{{UserID: assigneeID}}
	}

	chore := &chModel.Chore{
		CreatedBy:      createdBy,
		CircleID:       user.CircleID,
		Name:           choreRequest.Name,
		IsActive:       true,
		FrequencyType:  frequencyType,
		AssignStrategy: assignStrategy,
		AssignedTo:     assignedTo,
		Assignees:      assignees,
		Description:    choreRequest.Description,
		NextDueDate:    nextDueDate,
		CreatedAt:      time.Now().UTC(),
	}

	id, err := h.choreRepo.CreateChore(c, chore)
	if err != nil {
		log.Errorw("chore.api.CreateChore failed to create chore", "error", err)
		c.JSON(500, gin.H{"error": "Error creating chore"})
		return
	}

	// Fetch the created chore with all relations
	createdChore, err := h.choreRepo.GetChore(c, id, user.ID, user.CircleID)
	if err != nil {
		log.Errorw("chore.api.CreateChore failed to fetch created chore", "error", err)
		c.JSON(500, gin.H{"error": "Error fetching created chore"})
		return
	}

	h.eventProducer.ChoreCreated(c, user.WebhookURL, createdChore, &user.User)

	c.JSON(201, createdChore)
}

func (h *API) UpdateChore(c *gin.Context) {
	log := logging.FromContext(c)
	var choreRequest chModel.ChoreLiteReq
	user := auth.MustCurrentUser(c)

	choreIDRaw := c.Param("id")
	choreID, err := strconv.Atoi(choreIDRaw)
	if err != nil {
		log.Debugw("chore.api.UpdateChore failed to parse chore ID", "error", err)
		c.JSON(400, gin.H{"error": "Invalid chore ID"})
		return
	}

	if err := c.BindJSON(&choreRequest); err != nil {
		log.Debugw("chore.api.UpdateChore failed to bind JSON", "error", err)
		c.JSON(400, gin.H{"error": "Invalid request body"})
		return
	}

	// Get existing chore
	existingChore, err := h.choreRepo.GetChore(c, choreID, user.ID, user.CircleID)
	if err != nil {
		log.Errorw("chore.api.UpdateChore failed to get chore", "error", err)
		c.JSON(404, gin.H{"error": "Chore not found"})
		return
	}
	// get circle members:
	circleUsers, err := h.circleRepo.GetCircleUsers(c, user.CircleID)
	if err != nil {
		log.Errorw("chore.api.UpdateChore failed to get circle users", "error", err)
		c.JSON(500, gin.H{"error": "Failed to get circle members"})
		return
	}
	// Check if user owns this chore
	now := time.Now().UTC()
	if err := existingChore.CanEdit(user.ID, circleUsers, &now); err != nil {
		log.Debugw("chore.api.UpdateChore user does not own chore", "userID", user.ID, "choreCreatedBy", existingChore.CreatedBy)
		c.JSON(403, gin.H{"error": "You can only update your own chores"})
		return
	}

	// Validate required fields
	if choreRequest.Name == "" {
		c.JSON(400, gin.H{"error": "Chore name is required"})
		return
	}

	// Parse due date if provided
	var nextDueDate *time.Time
	if choreRequest.DueDate != "" {

		parsedDate, err := time.Parse(time.RFC3339, choreRequest.DueDate)
		if err != nil {
			parsedDateSimple, errSimple := time.Parse("2006-01-02", choreRequest.DueDate)
			if errSimple != nil {
				c.JSON(400, gin.H{"error": "Invalid due date format. Use RFC3339 or YYYY-MM-DD"})
				return
			}
			// Set time to now UTC
			now := time.Now().UTC()
			parsedDate = time.Date(parsedDateSimple.Year(), parsedDateSimple.Month(), parsedDateSimple.Day(), now.Hour(), now.Minute(), now.Second(), 0, time.UTC)
			err = nil
		}
		if err != nil {
			c.JSON(400, gin.H{"error": "Invalid due date format. Use RFC3339 format"})
			return
		}
		nextDueDate = &parsedDate
	}

	// Update only name, description and due date. A field the caller left out
	// keeps its value: nulling it would wipe a chore edited from Home Assistant.
	updates := map[string]interface{}{
		"name":       choreRequest.Name,
		"updated_by": user.ID,
		"updated_at": time.Now().UTC(),
	}
	if choreRequest.Description != nil {
		updates["description"] = choreRequest.Description
	}
	if nextDueDate != nil {
		updates["next_due_date"] = nextDueDate
	}

	err = h.choreRepo.UpdateChoreFields(c, choreID, updates)
	if err != nil {
		log.Errorw("chore.api.UpdateChore failed to update chore", "error", err)
		c.JSON(500, gin.H{"error": "Error updating chore"})
		return
	}

	// Fetch the updated chore
	updatedChore, err := h.choreRepo.GetChore(c, choreID, user.ID, user.CircleID)
	if err != nil {
		log.Errorw("chore.api.UpdateChore failed to fetch updated chore", "error", err)
		c.JSON(500, gin.H{"error": "Error fetching updated chore"})
		return
	}

	c.JSON(200, updatedChore)
}

func (h *API) CompleteChore(c *gin.Context) {
	log := logging.FromContext(c)
	completedDate := time.Now().UTC()
	choreIDRaw := c.Param("id")
	choreID, err := strconv.Atoi(choreIDRaw)
	if err != nil {
		log.Debugw("chore.api.CompleteChore failed to parse chore ID", "error", err)
		c.JSON(400, gin.H{
			"error": "Invalid ID",
		})
		return
	}
	completedByRaw := c.Query("completedBy")
	completedBy, err := strconv.Atoi(completedByRaw)

	currentUser := auth.MustCurrentUser(c)
	performer := currentUser.ID
	chore, err := h.choreRepo.GetChore(c, choreID, currentUser.ID, currentUser.CircleID)
	if err != nil {
		log.Errorw("chore.api.CompleteChore failed to get chore", "error", err)
		c.JSON(500, gin.H{
			"error": "Error getting chore",
		})
		return
	}

	// user need to be assigned to the chore to complete it
	circleUsers, err := h.circleRepo.GetCircleUsers(c, currentUser.CircleID)
	if err != nil {
		log.Errorw("Failed to retrieve circle users", "error", err)
		c.JSON(500, gin.H{
			"error": "Failed to retrieve circle users",
		})
		return
	}

	if completedBy != 0 && completedBy != currentUser.ID {
		if !currentUser.IsAdminOrManager(circleUsers) {
			log.Debugw("chore.api.CompleteChore unauthorized completedBy attempt", "userID", currentUser.ID, "completedBy", completedBy)
			c.JSON(403, gin.H{
				"error": "Only admins/managers can complete a chore on behalf of another member",
			})
			return
		}
		log.Debugw("chore.api.CompleteChore completedBy is set", "completedBy", completedBy)
		performer = completedBy
	}

	if !chore.CanComplete(performer, circleUsers) {
		log.Debugw("chore.api.CompleteChore user is not assigned to chore", "userID", performer, "choreID", choreID)
		c.JSON(400, gin.H{
			"error": "User is not assigned to chore",
		})
		return
	}

	// confirm that the chore in completion window:
	if chore.CompletionWindow != nil && chore.NextDueDate != nil {
		if completedDate.UTC().Before(chore.NextDueDate.UTC().Add(-time.Hour * time.Duration(*chore.CompletionWindow))) {
			log.Debugw("chore.api.CompleteChore chore is out of completion window", "choreID", choreID, "completionWindow", chore.CompletionWindow)
			c.JSON(400, gin.H{
				"error": "Chore is out of completion window",
			})
			return
		}
	}

	if chore.Status == chModel.ChoreStatusPendingApproval {
		c.JSON(409, gin.H{"error": "Chore is already waiting for approval"})
		return
	}

	// Check if chore requires approval
	if chore.RequireApproval {
		// Set chore status to pending approval instead of completing
		if err := h.choreRepo.SetChorePendingApproval(c, chore, nil, performer, &completedDate); err != nil {
			log.Errorw("chore.api.CompleteChore failed to set chore pending approval", "error", err)
			c.JSON(500, gin.H{
				"error": "Error setting chore pending approval",
			})
			return
		}

		updatedChore, err := h.choreRepo.GetChore(c, choreID, currentUser.ID, currentUser.CircleID)
		if err != nil {
			c.JSON(500, gin.H{
				"error": "Error getting chore",
			})
			return
		}

		c.JSON(200, gin.H{
			"res":     updatedChore,
			"message": "Chore completion submitted for approval",
		})
		return
	}

	choreHistory, err := h.choreRepo.GetChoreHistory(c, chore.ID)
	if err != nil {
		c.JSON(500, gin.H{
			"error": "Error getting chore history",
		})
		return
	}
	nextDueDate, nextAssignedTo, err := h.hh.planNext(c, chore, completedDate, performer, choreHistory)
	if err != nil {
		log.Errorw("chore.api.CompleteChore failed to plan next occurrence", "error", err)
		c.JSON(500, gin.H{"error": "Error scheduling next due date"})
		return
	}

	if err := h.choreRepo.CompleteChore(c, chore, nil, performer, nextDueDate, &completedDate, nextAssignedTo, true); err != nil {
		c.JSON(500, gin.H{
			"error": "Error completing chore",
		})
		return
	}
	if chore.SubTasks != nil && chore.FrequencyType != chModel.FrequencyTypeOnce {
		if err := h.stRepo.ResetSubtasksCompletion(c, chore.ID); err != nil {
			log.Errorw("chore.api.CompleteChore failed to reset subtasks completion", "error", err, "choreID", chore.ID)
		}
	}

	updatedChore, err := h.choreRepo.GetChore(c, choreID, currentUser.ID, currentUser.CircleID)
	if err != nil {
		c.JSON(500, gin.H{
			"error": "Error getting chore",
		})
		return
	}
	h.nPlanner.GenerateNotifications(c, updatedChore)
	h.eventProducer.ChoreCompleted(c, currentUser.WebhookURL, chore, &currentUser.User)
	c.JSON(200,
		updatedChore,
	)
}

func (h *API) SkipChore(c *gin.Context) {
	log := logging.FromContext(c)
	choreIDRaw := c.Param("id")
	choreID, err := strconv.Atoi(choreIDRaw)
	if err != nil {
		log.Debugw("chore.api.SkipChore failed to parse chore ID", "error", err)
		c.JSON(400, gin.H{"error": "Invalid ID"})
		return
	}

	currentUser := auth.MustCurrentUser(c)
	performer := currentUser.ID

	chore, err := h.choreRepo.GetChore(c, choreID, currentUser.ID, currentUser.CircleID)
	if err != nil {
		log.Errorw("chore.api.SkipChore failed to get chore", "error", err)
		c.JSON(500, gin.H{"error": "Error getting chore"})
		return
	}

	// user need to be assigned to the chore to skip it
	circleUsers, err := h.circleRepo.GetCircleUsers(c, currentUser.CircleID)
	if err != nil {
		log.Errorw("chore.api.SkipChore failed to retrieve circle users", "error", err)
		c.JSON(500, gin.H{"error": "Failed to retrieve circle users"})
		return
	}

	if completedByRaw := c.Query("completedBy"); completedByRaw != "" {
		if completedBy, errParse := strconv.Atoi(completedByRaw); errParse == nil && completedBy != 0 && completedBy != currentUser.ID {
			if !currentUser.IsAdminOrManager(circleUsers) {
				log.Debugw("chore.api.SkipChore unauthorized completedBy attempt", "userID", currentUser.ID, "completedBy", completedBy)
				c.JSON(403, gin.H{"error": "Only admins/managers can skip a chore on behalf of another member"})
				return
			}
			performer = completedBy
		}
	}

	if !chore.CanComplete(performer, circleUsers) {
		log.Debugw("chore.api.SkipChore user is not assigned to chore", "userID", performer, "choreID", choreID)
		c.JSON(400, gin.H{"error": "User is not assigned to chore"})
		return
	}

	if chore.NextDueDate == nil {
		c.JSON(400, gin.H{"error": "Chore has no due date to skip"})
		return
	}
	if chore.Status == chModel.ChoreStatusPendingApproval {
		c.JSON(409, gin.H{"error": "Chore is waiting for approval; approve or reject it first"})
		return
	}
	nextDueDate, err := scheduleNextDueDate(c, chore, chore.NextDueDate.UTC())
	if err == nil {
		nextDueDate, err = h.hh.finaliseDue(c, chore, nextDueDate)
	}
	if err != nil {
		log.Debugw("chore.api.SkipChore failed to schedule next due date", "error", err)
		c.JSON(500, gin.H{"error": "Error scheduling next due date"})
		return
	}
	nextAssignedTo := chore.AssignedTo
	if chore.AssignStrategy == chModel.AssignmentStrategyWeeklyRotation && nextDueDate != nil {
		nextAssignedTo = chSched.RotationAssignee(chore, *nextDueDate, localZone())
	}

	if err := h.choreRepo.SkipChore(c, chore, performer, nextDueDate, nextAssignedTo); err != nil {
		log.Errorw("chore.api.SkipChore failed to skip chore", "error", err)
		c.JSON(500, gin.H{"error": "Error skipping chore"})
		return
	}

	updatedChore, err := h.choreRepo.GetChore(c, choreID, currentUser.ID, currentUser.CircleID)
	if err != nil {
		c.JSON(500, gin.H{"error": "Error getting chore"})
		return
	}
	h.eventProducer.ChoreSkipped(c, currentUser.WebhookURL, updatedChore, &currentUser.User)
	c.JSON(200, updatedChore)
}

func (h *API) ApproveChore(c *gin.Context) {
	choreID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid ID"})
		return
	}
	currentUser := auth.MustCurrentUser(c)
	if !h.hh.requireManager(c) {
		return
	}
	chore, err := h.choreRepo.GetChore(c, choreID, currentUser.ID, currentUser.CircleID)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to retrieve chore"})
		return
	}
	updated, err := h.hh.approvePending(c, currentUser, chore)
	if err != nil {
		code, msg := errorStatus(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}
	c.JSON(200, updated)
}

func (h *API) RejectChore(c *gin.Context) {
	choreID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid ID"})
		return
	}
	currentUser := auth.MustCurrentUser(c)
	var req struct {
		Note string `json:"note"`
	}
	_ = c.ShouldBindJSON(&req)
	if !h.hh.requireManager(c) {
		return
	}
	chore, err := h.choreRepo.GetChore(c, choreID, currentUser.ID, currentUser.CircleID)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to retrieve chore"})
		return
	}
	var note *string
	if req.Note != "" {
		note = &req.Note
	}
	updated, err := h.hh.rejectPending(c, currentUser, chore, note)
	if err != nil {
		code, msg := errorStatus(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}
	c.JSON(200, updated)
}

func (h *API) GetCircleMembers(c *gin.Context) {
	currentUser := auth.MustCurrentUser(c)
	users, err := h.circleRepo.GetCircleUsers(c, currentUser.CircleID)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to get circle members"})
		return
	}
	if len(users) == 0 {
		c.JSON(404, gin.H{"error": "No members found in the circle"})
		return
	}
	c.JSON(200, users)
}
func (h *API) DeleteChore(c *gin.Context) {
	choreIDRaw := c.Param("id")
	choreID, err := strconv.Atoi(choreIDRaw)
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid chore ID"})
		return
	}
	currentUser := auth.MustCurrentUser(c)
	chore, err := h.choreRepo.GetChore(c, choreID, currentUser.ID, currentUser.CircleID)
	if err != nil {
		c.JSON(404, gin.H{"error": "Chore not found"})
		return
	}
	if chore.CreatedBy != currentUser.ID {
		c.JSON(403, gin.H{"error": "You can only delete your own chores"})
		return
	}
	if err := h.choreRepo.DeleteChore(c, choreID); err != nil {
		c.JSON(500, gin.H{"error": "Failed to delete chore"})
		return
	}
	c.JSON(200, gin.H{"message": "Chore deleted successfully"})
}

func APIs(cfg *config.Config, api *API, r *gin.Engine, auth *jwt.GinJWTMiddleware, limiter *limiter.Limiter, userRepo *uRepo.UserRepository) {

	tasksAPI := r.Group("eapi/v1/chore")
	tasksAPI.Use(
		utils.TimeoutMiddleware(cfg.Server.WriteTimeout),
		utils.RateLimitMiddleware(limiter),
		authMiddleware.APITokenMiddleware(userRepo),
	)
	{
		tasksAPI.GET("", api.GetAllChores)
		tasksAPI.GET("/:id", api.GetChore)
		tasksAPI.GET("/pending", api.hh.managerOnly, api.hh.getPending)
		tasksAPI.POST("/approve", api.hh.bulkHandler(true))
		tasksAPI.POST("/reject", api.hh.bulkHandler(false))
		tasksAPI.POST("", api.hh.managerOnly, api.CreateChore)
		tasksAPI.DELETE("/:id", api.hh.managerOnly, api.DeleteChore)
	}

	// Plus member only endpoints
	tasksPlusAPI := r.Group("eapi/v1/chore")
	tasksPlusAPI.Use(
		utils.TimeoutMiddleware(cfg.Server.WriteTimeout),
		utils.RateLimitMiddleware(limiter),
		authMiddleware.APITokenMiddleware(userRepo),
		authMiddleware.RequirePlusMemberMiddleware(cfg),
	)
	{
		tasksPlusAPI.POST("/:id/complete", api.CompleteChore)
		tasksPlusAPI.POST("/:id/skip", api.hh.managerOnly, api.SkipChore)
		tasksPlusAPI.POST("/:id/approve", api.ApproveChore)
		tasksPlusAPI.POST("/:id/reject", api.RejectChore)
		tasksPlusAPI.PUT("/:id", api.hh.managerOnly, api.UpdateChore)
	}

	circleAPI := r.Group("eapi/v1/circle")
	circleAPI.Use(
		utils.TimeoutMiddleware(cfg.Server.WriteTimeout),
		utils.RateLimitMiddleware(limiter),
		authMiddleware.APITokenMiddleware(userRepo),
		authMiddleware.RequirePlusMemberMiddleware(cfg),
	)
	{
		circleAPI.GET("/members", api.GetCircleMembers)
	}

	householdAPI := r.Group("eapi/v1/household")
	householdAPI.Use(
		utils.TimeoutMiddleware(cfg.Server.WriteTimeout),
		utils.RateLimitMiddleware(limiter),
		authMiddleware.APITokenMiddleware(userRepo),
	)
	{
		householdAPI.GET("/checkpoints", api.hh.getCheckpoints)
	}

}
