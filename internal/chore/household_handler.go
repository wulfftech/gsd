package chore

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"

	auth "donetick.com/core/internal/auth"
	chModel "donetick.com/core/internal/chore/model"
	chSched "donetick.com/core/internal/chore/scheduling"
	circle "donetick.com/core/internal/circle/model"
	"donetick.com/core/internal/household"
	stModel "donetick.com/core/internal/subtask/model"
	uModel "donetick.com/core/internal/user/model"
	"donetick.com/core/logging"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ---------------------------------------------------------------- pending

type pendingItem struct {
	ChoreID         int                `json:"choreId"`
	Name            string             `json:"name"`
	Checkpoint      chModel.Checkpoint `json:"checkpoint"`
	Points          *int               `json:"points"`
	AssignedTo      *int               `json:"assignedTo"`
	CompletedBy     int                `json:"completedBy"`
	CompletedByName string             `json:"completedByName"`
	PerformedAt     *time.Time         `json:"performedAt"`
	DueDate         *time.Time         `json:"dueDate"`
	Note            *string            `json:"note"`
	HistoryID       int                `json:"historyId"`
}

func (h *Handler) pendingItems(ctx context.Context, circleID int) ([]pendingItem, error) {
	rows, err := h.choreRepo.GetPendingApprovals(ctx, circleID)
	if err != nil {
		return nil, err
	}
	names := h.memberNames(ctx, circleID)
	out := make([]pendingItem, 0, len(rows))
	for _, r := range rows {
		it := pendingItem{
			ChoreID:    r.Chore.ID,
			Name:       r.Chore.Name,
			Checkpoint: r.Chore.Checkpoint,
			Points:     r.Chore.Points,
			AssignedTo: r.Chore.AssignedTo,
			DueDate:    r.Chore.NextDueDate,
		}
		if r.History != nil {
			it.CompletedBy = r.History.CompletedBy
			it.CompletedByName = names[r.History.CompletedBy]
			it.PerformedAt = r.History.PerformedAt
			it.Note = r.History.Note
			it.HistoryID = r.History.ID
		}
		out = append(out, it)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].CompletedBy != out[j].CompletedBy {
			return out[i].CompletedBy < out[j].CompletedBy
		}
		a, b := out[i].PerformedAt, out[j].PerformedAt
		if a == nil || b == nil {
			return b != nil
		}
		return a.Before(*b)
	})
	return out, nil
}

func (h *Handler) memberNames(ctx context.Context, circleID int) map[int]string {
	names := map[int]string{}
	users, err := h.circleRepo.GetCircleUsers(ctx, circleID)
	if err != nil {
		return names
	}
	for _, u := range users {
		names[u.UserID] = u.DisplayName
	}
	return names
}

func (h *Handler) getPending(c *gin.Context) {
	user := auth.MustCurrentUser(c)
	items, err := h.pendingItems(c, user.CircleID)
	if err != nil {
		logging.FromContext(c).Errorw("failed to list pending approvals", "error", err)
		c.JSON(500, gin.H{"error": "Failed to list pending approvals"})
		return
	}
	c.JSON(200, gin.H{"res": items})
}

// ---------------------------------------------------------------- bulk

type bulkFailure struct {
	ChoreID int    `json:"choreId"`
	Error   string `json:"error"`
}

type bulkResult struct {
	Done   []int         `json:"done"`
	Failed []bulkFailure `json:"failed"`
}

type bulkReq struct {
	ChoreIDs []int  `json:"choreIds"`
	All      bool   `json:"all"`
	Note     string `json:"note"`
}

// bulkAct approves or rejects several chores. Each one is settled on its own,
// so one failure doesn't stop the rest.
func (h *Handler) bulkAct(ctx context.Context, approver *uModel.UserDetails, ids []int, approve bool, note *string) bulkResult {
	res := bulkResult{Done: []int{}, Failed: []bulkFailure{}}
	seen := map[int]bool{}
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		chore, err := h.choreRepo.GetChore(ctx, id, approver.ID, approver.CircleID)
		if err != nil {
			res.Failed = append(res.Failed, bulkFailure{ChoreID: id, Error: "Chore not found"})
			continue
		}
		if approve {
			_, err = h.approvePending(ctx, approver, chore)
		} else {
			_, err = h.rejectPending(ctx, approver, chore, note)
		}
		if err != nil {
			_, msg := errorStatus(err)
			logging.FromContext(ctx).Errorw("bulk action failed", "error", err, "choreID", id, "approve", approve)
			res.Failed = append(res.Failed, bulkFailure{ChoreID: id, Error: msg})
			continue
		}
		res.Done = append(res.Done, id)
	}
	return res
}

func (h *Handler) bulkHandler(approve bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		user := auth.MustCurrentUser(c)
		if !h.requireManager(c) {
			return
		}
		var req bulkReq
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": "Invalid request body"})
			return
		}
		ids := req.ChoreIDs
		if req.All {
			if !approve {
				c.JSON(400, gin.H{"error": "Reject needs a list of chores"})
				return
			}
			items, err := h.pendingItems(c, user.CircleID)
			if err != nil {
				c.JSON(500, gin.H{"error": "Failed to list pending approvals"})
				return
			}
			ids = nil
			for _, it := range items {
				ids = append(ids, it.ChoreID)
			}
		} else if len(ids) == 0 {
			c.JSON(400, gin.H{"error": "No chores given"})
			return
		}
		var note *string
		if req.Note != "" {
			note = &req.Note
		}
		c.JSON(200, gin.H{"res": h.bulkAct(c, user, ids, approve, note)})
	}
}

// ---------------------------------------------------------------- the day

type checkpointState struct {
	State     string `json:"state"`
	Total     int    `json:"total"`
	Approved  int    `json:"approved"`
	Pending   int    `json:"pending"`
	Remaining int    `json:"remaining"`
}

type kidCheckpoints struct {
	UserID      int                         `json:"userId"`
	Name        string                      `json:"name"`
	Checkpoints map[string]*checkpointState `json:"checkpoints"`
}

type checkpointsRes struct {
	Date string            `json:"date"`
	Kids []*kidCheckpoints `json:"kids"`
	All  map[string]bool   `json:"all"`
}

// kidJob is one job for today, as the kiosk shows it.
type kidJob struct {
	ChoreID    int                `json:"choreId"`
	Name       string             `json:"name"`
	Checkpoint chModel.Checkpoint `json:"checkpoint"`
	State      string             `json:"state"` // todo, pending, done
	DueDate    *time.Time         `json:"dueDate"`
	Steps      []string           `json:"steps"`
	TickedBy   int                `json:"tickedBy,omitempty"`
	Points     *int               `json:"points,omitempty"`
	Redo       bool               `json:"redo,omitempty"` // a parent sent it back today
}

type householdDay struct {
	date   time.Time
	kids   []*circle.UserCircleDetail
	jobs   map[int][]*kidJob // by kid user id
	extras []*kidJob
	chores map[int]*chModel.Chore
}

// kidsOf are the circle's active members with the member role, by user id.
func kidsOf(users []*circle.UserCircleDetail) []*circle.UserCircleDetail {
	var kids []*circle.UserCircleDetail
	for _, u := range users {
		if u.Role == circle.UserRoleMember && u.IsActive {
			kids = append(kids, u)
		}
	}
	sort.Slice(kids, func(i, j int) bool { return kids[i].UserID < kids[j].UserID })
	return kids
}

func choreSteps(ch *chModel.Chore) []string {
	out := []string{}
	if ch.SubTasks == nil {
		return out
	}
	st := append(([]stModel.SubTask)(nil), *ch.SubTasks...)
	sort.SliceStable(st, func(i, j int) bool { return st[i].OrderID < st[j].OrderID })
	for _, s := range st {
		out = append(out, s.Name)
	}
	return out
}

// loadDay gathers today's checkpoint jobs and open extras for every kid.
// A job is today's when it's due before the end of today (including one left
// over from yesterday that midnight hasn't rolled yet), or when its occurrence
// due today has already been approved.
func (h *Handler) loadDay(ctx context.Context, circleID int, now time.Time) (*householdDay, error) {
	loc := localZone()
	start := chSched.StartOfDay(now, loc)
	end := start.AddDate(0, 0, 1)

	users, err := h.circleRepo.GetCircleUsers(ctx, circleID)
	if err != nil {
		return nil, err
	}
	chores, err := h.choreRepo.GetHouseholdChores(ctx, circleID)
	if err != nil {
		return nil, err
	}
	day := &householdDay{date: start, kids: kidsOf(users), jobs: map[int][]*kidJob{}, chores: map[int]*chModel.Chore{}}
	ids := make([]int, 0, len(chores))
	for _, ch := range chores {
		ids = append(ids, ch.ID)
		day.chores[ch.ID] = ch
	}
	approvedToday, err := h.choreRepo.CompletedHistoryDueBetween(ctx, ids, start, end)
	if err != nil {
		return nil, err
	}
	pendingBy := map[int]int{}
	for _, ch := range chores {
		if ch.Status == chModel.ChoreStatusPendingApproval {
			if ph, err := h.choreRepo.GetLatestPendingHistory(ctx, ch.ID); err == nil {
				pendingBy[ch.ID] = ph.CompletedBy
			}
		}
	}
	for _, ch := range chores {
		state := "todo"
		if ch.Status == chModel.ChoreStatusPendingApproval {
			state = "pending"
		}
		if ch.Checkpoint == chModel.CheckpointExtra {
			// Extras are open to any kid until done; one with a due date shows from that day.
			if ch.NextDueDate != nil && !ch.NextDueDate.Before(end) {
				continue
			}
			day.extras = append(day.extras, &kidJob{ChoreID: ch.ID, Name: ch.Name, Checkpoint: ch.Checkpoint, State: state,
				DueDate: ch.NextDueDate, Steps: choreSteps(ch), TickedBy: pendingBy[ch.ID], Points: ch.Points})
			continue
		}
		if ch.AssignedTo == nil || ch.NextDueDate == nil || !ch.NextDueDate.Before(end) {
			continue
		}
		kid := *ch.AssignedTo
		day.jobs[kid] = append(day.jobs[kid], &kidJob{ChoreID: ch.ID, Name: ch.Name, Checkpoint: ch.Checkpoint,
			State: state, DueDate: ch.NextDueDate, Steps: choreSteps(ch), TickedBy: pendingBy[ch.ID]})
	}
	for _, hst := range approvedToday {
		ch := day.chores[hst.ChoreID]
		if ch == nil || ch.Checkpoint == chModel.CheckpointExtra {
			continue
		}
		kid := hst.CompletedBy
		if hst.AssignedTo != nil && *hst.AssignedTo != 0 {
			kid = *hst.AssignedTo
		}
		day.jobs[kid] = append(day.jobs[kid], &kidJob{ChoreID: ch.ID, Name: ch.Name, Checkpoint: ch.Checkpoint,
			State: "done", DueDate: hst.DueDate, Steps: choreSteps(ch), TickedBy: hst.CompletedBy})
	}
	var todo []int
	for _, js := range day.jobs {
		for _, j := range js {
			if j.State == "todo" {
				todo = append(todo, j.ChoreID)
			}
		}
	}
	for _, j := range day.extras {
		if j.State == "todo" {
			todo = append(todo, j.ChoreID)
		}
	}
	if redo, err := h.choreRepo.RejectedSince(ctx, todo, start); err == nil {
		for _, js := range day.jobs {
			for _, j := range js {
				j.Redo = j.State == "todo" && redo[j.ChoreID]
			}
		}
		for _, j := range day.extras {
			j.Redo = j.State == "todo" && redo[j.ChoreID]
		}
	}
	for _, js := range day.jobs {
		sortJobs(js)
	}
	return day, nil
}

func sortJobs(js []*kidJob) {
	sort.SliceStable(js, func(i, j int) bool {
		oi, oj := checkpointOrder(js[i].Checkpoint), checkpointOrder(js[j].Checkpoint)
		if oi != oj {
			return oi < oj
		}
		// What's left first, then waiting, then done.
		si, sj := stateOrder(js[i].State), stateOrder(js[j].State)
		if si != sj {
			return si < sj
		}
		a, b := js[i].DueDate, js[j].DueDate
		if a != nil && b != nil && !a.Equal(*b) {
			return a.Before(*b)
		}
		return js[i].Name < js[j].Name
	})
}

func stateOrder(s string) int {
	switch s {
	case "todo":
		return 0
	case "pending":
		return 1
	}
	return 2
}

func checkpointOrder(c chModel.Checkpoint) int {
	switch c {
	case chModel.CheckpointMorning:
		return 0
	case chModel.CheckpointAfternoon:
		return 1
	case chModel.CheckpointEvening:
		return 2
	}
	return 3
}

func (d *householdDay) checkpoints() *checkpointsRes {
	res := &checkpointsRes{Date: d.date.Format(household.DateLayout), Kids: []*kidCheckpoints{}, All: map[string]bool{}}
	for _, cp := range chModel.GatedCheckpoints {
		res.All[string(cp)] = true
	}
	for _, kid := range d.kids {
		kc := &kidCheckpoints{UserID: kid.UserID, Name: kid.DisplayName, Checkpoints: map[string]*checkpointState{}}
		for _, cp := range chModel.GatedCheckpoints {
			st := &checkpointState{}
			for _, j := range d.jobs[kid.UserID] {
				if j.Checkpoint != cp {
					continue
				}
				switch j.State {
				case "done":
					st.Approved++
				case "pending":
					st.Pending++
				default:
					st.Remaining++
				}
			}
			st.Total = st.Approved + st.Pending + st.Remaining
			switch {
			case st.Total == 0:
				st.State = "none"
			case st.Remaining > 0:
				st.State = "todo"
			case st.Pending > 0:
				st.State = "waiting"
			default:
				st.State = "done"
			}
			if st.State != "none" && st.State != "done" {
				res.All[string(cp)] = false
			}
			kc.Checkpoints[string(cp)] = st
		}
		res.Kids = append(res.Kids, kc)
	}
	return res
}

func (h *Handler) getCheckpoints(c *gin.Context) {
	user := auth.MustCurrentUser(c)
	day, err := h.loadDay(c, user.CircleID, nowFunc())
	if err != nil {
		logging.FromContext(c).Errorw("failed to compute checkpoints", "error", err)
		c.JSON(500, gin.H{"error": "Failed to compute checkpoints"})
		return
	}
	c.JSON(200, gin.H{"res": day.checkpoints()})
}

// ---------------------------------------------------------------- holidays

func (h *Handler) listHolidays(c *gin.Context) {
	user := auth.MustCurrentUser(c)
	hs, err := h.hRepo.ListHolidays(c, user.CircleID)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to list holidays"})
		return
	}
	if hs == nil {
		hs = []*household.Holiday{}
	}
	c.JSON(200, gin.H{"res": hs})
}

func (h *Handler) createHoliday(c *gin.Context) {
	user := auth.MustCurrentUser(c)
	var hol household.Holiday
	if err := c.ShouldBindJSON(&hol); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request body"})
		return
	}
	hol.ID = 0
	hol.CircleID = user.CircleID
	hol.Name = strings.TrimSpace(hol.Name)
	if err := household.ValidateHoliday(&hol); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if err := h.hRepo.CreateHoliday(c, &hol); err != nil {
		c.JSON(500, gin.H{"error": "Failed to save holiday"})
		return
	}
	c.JSON(200, gin.H{"res": hol})
}

func (h *Handler) deleteHoliday(c *gin.Context) {
	user := auth.MustCurrentUser(c)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid ID"})
		return
	}
	if err := h.hRepo.DeleteHoliday(c, user.CircleID, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(404, gin.H{"error": "Holiday not found"})
			return
		}
		c.JSON(500, gin.H{"error": "Failed to delete holiday"})
		return
	}
	c.JSON(200, gin.H{"res": "ok"})
}

// ---------------------------------------------------------------- kiosk setup

func (h *Handler) getKioskStatus(c *gin.Context) {
	user := auth.MustCurrentUser(c)
	tokenSet, pinSet, err := h.hRepo.KioskStatus(c, user.CircleID)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to read kiosk settings"})
		return
	}
	c.JSON(200, gin.H{"res": gin.H{"tokenSet": tokenSet, "pinSet": pinSet}})
}

func (h *Handler) newKioskToken(c *gin.Context) {
	user := auth.MustCurrentUser(c)
	token, err := h.hRepo.NewKioskToken(c, user.CircleID)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to create kiosk link"})
		return
	}
	h.kiosk.revokeCircle(user.CircleID)
	c.JSON(200, gin.H{"res": gin.H{"token": token, "path": "/kiosk/" + token}})
}

func (h *Handler) setKioskPIN(c *gin.Context) {
	user := auth.MustCurrentUser(c)
	var req struct {
		Pin string `json:"pin"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || !household.ValidPIN(req.Pin) {
		c.JSON(400, gin.H{"error": "PIN must be 4 to 8 digits"})
		return
	}
	if err := h.hRepo.SetParentPIN(c, user.CircleID, req.Pin); err != nil {
		c.JSON(500, gin.H{"error": "Failed to save PIN"})
		return
	}
	h.kiosk.revokeCircle(user.CircleID)
	c.JSON(200, gin.H{"res": "ok"})
}

// HouseholdRoutes registers the household endpoints and the kiosk.
func HouseholdRoutes(router *gin.Engine, h *Handler, multiAuthMiddleware *auth.MultiAuthMiddleware) {
	g := router.Group("api/v1/household")
	g.Use(multiAuthMiddleware.MiddlewareFunc())
	{
		g.GET("/checkpoints", h.getCheckpoints)
		g.GET("/holidays", h.listHolidays)
		g.POST("/holidays", h.managerOnly, h.createHoliday)
		g.DELETE("/holidays/:id", h.managerOnly, h.deleteHoliday)
		g.GET("/kiosk", h.managerOnly, h.getKioskStatus)
		g.POST("/kiosk/token", h.managerOnly, h.newKioskToken)
		g.PUT("/kiosk/pin", h.managerOnly, h.setKioskPIN)
	}
	ch := router.Group("api/v1/chores")
	ch.Use(multiAuthMiddleware.MiddlewareFunc())
	{
		ch.GET("/pending", h.managerOnly, h.getPending)
		ch.POST("/approve", h.bulkHandler(true))
		ch.POST("/reject", h.bulkHandler(false))
	}
	KioskRoutes(router, h)
}
