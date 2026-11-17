package chore

import (
	"context"
	"crypto/rand"
	_ "embed"
	"encoding/hex"
	"errors"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	chModel "donetick.com/core/internal/chore/model"
	chSched "donetick.com/core/internal/chore/scheduling"
	circle "donetick.com/core/internal/circle/model"
	uModel "donetick.com/core/internal/user/model"
	"donetick.com/core/logging"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// The shared chores kiosk: a page for an old tablet in a common room. Kids tap
// their name and tick jobs; a parent unlocks approvals with a PIN. The page is
// plain ES5 and XMLHttpRequest so it runs on Safari 12.
//
// Auth is a per-circle kiosk token in the URL. It can only read today's jobs,
// tick and untick checkpoint jobs for kids, and, after a PIN unlock, approve
// and reject. It can't reach anything else.

//go:embed kiosk.html
var kioskPage []byte

const (
	parentSessionTTL = 2 * time.Minute
	pinMaxFailures   = 5
	pinLockout       = 5 * time.Minute
)

// Screen windows. These mirror the Home Assistant automation that switches the
// living-room TV; the kiosk only describes them.
var (
	screensMorningFrom   = 7*time.Hour + 30*time.Minute
	screensAfternoonFrom = 15 * time.Hour
	screensUngatedFrom   = 19 * time.Hour
)

type parentSession struct {
	circleID int
	expires  time.Time
}

type pinState struct {
	failures    int
	lockedUntil time.Time
}

type kioskSessions struct {
	mu       sync.Mutex
	sessions map[string]*parentSession
	pins     map[int]*pinState
}

func newKioskSessions() *kioskSessions {
	return &kioskSessions{sessions: map[string]*parentSession{}, pins: map[int]*pinState{}}
}

func (k *kioskSessions) open(circleID int, now time.Time) string {
	b := make([]byte, 24)
	_, _ = rand.Read(b)
	id := hex.EncodeToString(b)
	k.mu.Lock()
	defer k.mu.Unlock()
	for sid, s := range k.sessions {
		if now.After(s.expires) {
			delete(k.sessions, sid)
		}
	}
	k.sessions[id] = &parentSession{circleID: circleID, expires: now.Add(parentSessionTTL)}
	return id
}

// touch validates a parent session and slides its expiry.
func (k *kioskSessions) touch(id string, circleID int, now time.Time) bool {
	k.mu.Lock()
	defer k.mu.Unlock()
	s, ok := k.sessions[id]
	if !ok || s.circleID != circleID || now.After(s.expires) {
		delete(k.sessions, id)
		return false
	}
	s.expires = now.Add(parentSessionTTL)
	return true
}

func (k *kioskSessions) close(id string) {
	k.mu.Lock()
	defer k.mu.Unlock()
	delete(k.sessions, id)
}

func (k *kioskSessions) revokeCircle(circleID int) {
	k.mu.Lock()
	defer k.mu.Unlock()
	for id, s := range k.sessions {
		if s.circleID == circleID {
			delete(k.sessions, id)
		}
	}
	delete(k.pins, circleID)
}

// pinAllowed reports whether a PIN attempt may be made now, and until when not.
func (k *kioskSessions) pinAllowed(circleID int, now time.Time) (bool, time.Time) {
	k.mu.Lock()
	defer k.mu.Unlock()
	p := k.pins[circleID]
	if p == nil || now.After(p.lockedUntil) {
		return true, time.Time{}
	}
	return false, p.lockedUntil
}

func (k *kioskSessions) pinResult(circleID int, ok bool, now time.Time) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if ok {
		delete(k.pins, circleID)
		return
	}
	p := k.pins[circleID]
	if p == nil {
		p = &pinState{}
		k.pins[circleID] = p
	}
	p.failures++
	if p.failures >= pinMaxFailures {
		p.failures = 0
		p.lockedUntil = now.Add(pinLockout)
	}
}

// ---------------------------------------------------------------- middleware

const kioskCircleKey = "kioskCircleID"

func (h *Handler) kioskAuth(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.Header("Referrer-Policy", "no-referrer")
	c.Header("X-Robots-Tag", "noindex")
	circleID, err := h.hRepo.CircleForKioskToken(c, c.Param("token"))
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			logging.FromContext(c).Errorw("kiosk token lookup failed", "error", err)
		}
		msg := "This kiosk link isn't valid any more. Ask a parent to set it up again."
		if c.Request.Method == "GET" && strings.Count(strings.Trim(c.Request.URL.Path, "/"), "/") == 1 {
			// The page itself: a person is looking at this, not the script.
			c.Data(404, "text/html; charset=utf-8", []byte(`<!DOCTYPE html><meta name="viewport" content="width=device-width, initial-scale=1"><title>Jobs</title>`+
				`<body style="font:20px -apple-system,Helvetica,Arial,sans-serif;background:#f4f1ea;color:#23201b;text-align:center;padding:80px 30px">`+msg+`</body>`))
			c.Abort()
			return
		}
		c.AbortWithStatusJSON(404, gin.H{"error": msg})
		return
	}
	c.Set(kioskCircleKey, circleID)
	c.Next()
}

func (h *Handler) kioskParentOnly(c *gin.Context) {
	circleID := c.GetInt(kioskCircleKey)
	if !h.kiosk.touch(c.GetHeader("X-Kiosk-Parent"), circleID, nowFunc()) {
		c.AbortWithStatusJSON(401, gin.H{"error": "Parent mode has locked. Enter the PIN again."})
		return
	}
	c.Next()
}

// kioskActor is the account kiosk actions are recorded against: the circle's
// longest-standing admin. The kid who ticked is recorded as the completer.
func (h *Handler) kioskActor(ctx context.Context, circleID int) (*uModel.UserDetails, []*circle.UserCircleDetail, error) {
	users, err := h.circleRepo.GetCircleUsers(ctx, circleID)
	if err != nil {
		return nil, nil, err
	}
	var admins []*circle.UserCircleDetail
	for _, u := range users {
		if u.Role == circle.UserRoleAdmin && u.IsActive {
			admins = append(admins, u)
		}
	}
	if len(admins) == 0 {
		return nil, nil, errors.New("circle has no admin")
	}
	sort.Slice(admins, func(i, j int) bool { return admins[i].UserID < admins[j].UserID })
	u, err := h.uRepo.GetUserByID(ctx, admins[0].UserID)
	if err != nil {
		return nil, nil, err
	}
	u.CircleID = circleID
	return &uModel.UserDetails{User: *u}, users, nil
}

// ---------------------------------------------------------------- state

type kioskKid struct {
	UserID      int                         `json:"userId"`
	Name        string                      `json:"name"`
	Jobs        []*kidJob                   `json:"jobs"`
	Checkpoints map[string]*checkpointState `json:"checkpoints"`
}

type kioskRole struct {
	Role   string `json:"role"`
	UserID int    `json:"userId"`
	Name   string `json:"name"`
}

type kioskState struct {
	Date     string          `json:"date"`
	Now      time.Time       `json:"now"`
	Kids     []*kioskKid     `json:"kids"`
	Extras   []*kidJob       `json:"extras"`
	Roles    []kioskRole     `json:"roles"`
	All      map[string]bool `json:"all"`
	Screens  string          `json:"screens"`
	Unlocked bool            `json:"unlocked"`
	PinSet   bool            `json:"pinSet"`
	Pending  int             `json:"pending"`
}

// roleName is the part of a rotation chore's name before ":" or " - ":
// "Dishes: empty the dishwasher" belongs to the Dishes role.
func roleName(name string) string {
	for _, sep := range []string{":", " - ", " – "} {
		if i := strings.Index(name, sep); i > 0 {
			return strings.TrimSpace(name[:i])
		}
	}
	return strings.TrimSpace(name)
}

func screensMessage(now time.Time, all map[string]bool) (string, bool) {
	loc := localZone()
	since := now.In(loc).Sub(chSched.StartOfDay(now, loc))
	switch {
	case since < screensMorningFrom:
		if all["morning"] {
			return "Morning jobs are done. The TV comes on at 7:30.", false
		}
		return "The TV comes on from 7:30 once everyone's morning jobs are approved.", false
	case since < screensAfternoonFrom:
		if all["morning"] {
			return "Morning jobs are done. The TV is on.", true
		}
		return "The TV comes on when everyone's morning jobs are approved.", false
	case since < screensUngatedFrom:
		if all["afternoon"] {
			return "After-school jobs are done. The TV is on.", true
		}
		return "The TV comes on when everyone's after-school jobs are approved, or at 7pm.", false
	default:
		return "Evening: the TV is on its normal schedule.", true
	}
}

func (h *Handler) kioskStateFor(ctx context.Context, circleID int) (*kioskState, error) {
	now := nowFunc()
	day, err := h.loadDay(ctx, circleID, now)
	if err != nil {
		return nil, err
	}
	cps := day.checkpoints()
	byKid := map[int]*kidCheckpoints{}
	for _, k := range cps.Kids {
		byKid[k.UserID] = k
	}
	st := &kioskState{Date: cps.Date, Now: now.UTC(), Kids: []*kioskKid{}, Extras: day.extras, All: cps.All, Roles: []kioskRole{}}
	if st.Extras == nil {
		st.Extras = []*kidJob{}
	}
	names := map[int]string{}
	for _, kid := range day.kids {
		names[kid.UserID] = kid.DisplayName
		jobs := day.jobs[kid.UserID]
		if jobs == nil {
			jobs = []*kidJob{}
		}
		st.Kids = append(st.Kids, &kioskKid{UserID: kid.UserID, Name: kid.DisplayName, Jobs: jobs, Checkpoints: byKid[kid.UserID].Checkpoints})
	}
	rot, err := h.choreRepo.GetRotationChores(ctx, circleID)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, ch := range rot {
		who := chSched.RotationAssignee(ch, now, localZone())
		r := roleName(ch.Name)
		if who == nil || seen[r] {
			continue
		}
		seen[r] = true
		st.Roles = append(st.Roles, kioskRole{Role: r, UserID: *who, Name: names[*who]})
	}
	st.Screens, st.Unlocked = screensMessage(now, cps.All)
	if _, pinSet, err := h.hRepo.KioskStatus(ctx, circleID); err == nil {
		st.PinSet = pinSet
	}
	if p, err := h.choreRepo.GetPendingApprovals(ctx, circleID); err == nil {
		st.Pending = len(p)
	}
	return st, nil
}

func (h *Handler) kioskGetPage(c *gin.Context) {
	c.Header("Content-Security-Policy", "default-src 'self'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; img-src 'self' data:; connect-src 'self'")
	c.Data(200, "text/html; charset=utf-8", kioskPage)
}

func (h *Handler) kioskGetState(c *gin.Context) {
	st, err := h.kioskStateFor(c, c.GetInt(kioskCircleKey))
	if err != nil {
		logging.FromContext(c).Errorw("kiosk state failed", "error", err)
		c.JSON(500, gin.H{"error": "Couldn't load today's jobs"})
		return
	}
	c.JSON(200, gin.H{"res": st})
}

type kioskTickReq struct {
	ChoreID int `json:"choreId"`
	UserID  int `json:"userId"`
}

// kioskChoreFor loads a chore the kiosk may act on for a kid, or explains why not.
func (h *Handler) kioskChoreFor(ctx context.Context, circleID int, req kioskTickReq) (*chModel.Chore, *uModel.UserDetails, error) {
	actor, users, err := h.kioskActor(ctx, circleID)
	if err != nil {
		return nil, nil, err
	}
	isKid := false
	for _, k := range kidsOf(users) {
		if k.UserID == req.UserID {
			isKid = true
		}
	}
	if !isKid {
		return nil, nil, newStatusError(400, "Tap your name first")
	}
	chore, err := h.choreRepo.GetChore(ctx, req.ChoreID, actor.ID, circleID)
	if err != nil {
		return nil, nil, newStatusError(404, "That job has gone. Pull down to refresh.")
	}
	if !chore.IsActive || chore.Checkpoint == chModel.CheckpointNone {
		return nil, nil, newStatusError(400, "That job can't be ticked here")
	}
	if chore.Checkpoint == chModel.CheckpointExtra {
		if chore.AssignedTo != nil && *chore.AssignedTo != 0 && *chore.AssignedTo != req.UserID {
			return nil, nil, newStatusError(400, "That extra belongs to someone else")
		}
		return chore, actor, nil
	}
	if chore.AssignedTo == nil || *chore.AssignedTo != req.UserID {
		return nil, nil, newStatusError(400, "That's not your job")
	}
	end := chSched.StartOfDay(nowFunc(), localZone()).AddDate(0, 0, 1)
	if chore.NextDueDate == nil || !chore.NextDueDate.Before(end) {
		return nil, nil, newStatusError(400, "That job isn't due today")
	}
	return chore, actor, nil
}

func (h *Handler) kioskTick(c *gin.Context) {
	var req kioskTickReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request"})
		return
	}
	circleID := c.GetInt(kioskCircleKey)
	chore, actor, err := h.kioskChoreFor(c, circleID, req)
	if err == nil {
		_, err = h.submitForApproval(c, actor, chore, req.UserID, nowFunc().UTC())
	}
	if err != nil {
		code, msg := errorStatus(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}
	h.kioskGetState(c)
}

func (h *Handler) kioskUntick(c *gin.Context) {
	var req kioskTickReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request"})
		return
	}
	circleID := c.GetInt(kioskCircleKey)
	chore, _, err := h.kioskChoreFor(c, circleID, req)
	if err != nil {
		code, msg := errorStatus(err)
		c.JSON(code, gin.H{"error": msg})
		return
	}
	if chore.Status != chModel.ChoreStatusPendingApproval {
		c.JSON(400, gin.H{"error": "That job isn't ticked"})
		return
	}
	ph, err := h.choreRepo.GetLatestPendingHistory(c, chore.ID)
	if err != nil {
		c.JSON(400, gin.H{"error": "That job isn't ticked"})
		return
	}
	if ph.CompletedBy != req.UserID {
		c.JSON(403, gin.H{"error": "Only the person who ticked it can untick it"})
		return
	}
	if err := h.choreRepo.WithdrawPending(c, chore.ID, ph.ID); err != nil {
		c.JSON(500, gin.H{"error": "Couldn't untick that job"})
		return
	}
	h.kioskGetState(c)
}

func (h *Handler) kioskUnlock(c *gin.Context) {
	var req struct {
		Pin string `json:"pin"`
	}
	_ = c.ShouldBindJSON(&req)
	circleID := c.GetInt(kioskCircleKey)
	now := nowFunc()
	if ok, until := h.kiosk.pinAllowed(circleID, now); !ok {
		mins := int(until.Sub(now).Minutes()) + 1
		c.JSON(429, gin.H{"error": "Too many wrong PINs. Try again in " + strconv.Itoa(mins) + " min."})
		return
	}
	ok, err := h.hRepo.CheckParentPIN(c, circleID, req.Pin)
	if err != nil {
		c.JSON(500, gin.H{"error": "Couldn't check the PIN"})
		return
	}
	h.kiosk.pinResult(circleID, ok, now)
	if !ok {
		c.JSON(401, gin.H{"error": "Wrong PIN"})
		return
	}
	c.JSON(200, gin.H{"res": gin.H{"session": h.kiosk.open(circleID, now), "ttlSeconds": int(parentSessionTTL.Seconds())}})
}

func (h *Handler) kioskLock(c *gin.Context) {
	h.kiosk.close(c.GetHeader("X-Kiosk-Parent"))
	c.JSON(200, gin.H{"res": "ok"})
}

func (h *Handler) kioskPending(c *gin.Context) {
	items, err := h.pendingItems(c, c.GetInt(kioskCircleKey))
	if err != nil {
		c.JSON(500, gin.H{"error": "Couldn't load approvals"})
		return
	}
	c.JSON(200, gin.H{"res": items})
}

func (h *Handler) kioskBulk(approve bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req bulkReq
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": "Invalid request"})
			return
		}
		circleID := c.GetInt(kioskCircleKey)
		actor, _, err := h.kioskActor(c, circleID)
		if err != nil {
			c.JSON(500, gin.H{"error": "Couldn't act for the household"})
			return
		}
		ids := req.ChoreIDs
		if req.All && approve {
			items, err := h.pendingItems(c, circleID)
			if err != nil {
				c.JSON(500, gin.H{"error": "Couldn't load approvals"})
				return
			}
			ids = nil
			for _, it := range items {
				ids = append(ids, it.ChoreID)
			}
		}
		var note *string
		if req.Note != "" {
			note = &req.Note
		}
		c.JSON(200, gin.H{"res": h.bulkAct(c, actor, ids, approve, note)})
	}
}

// KioskRoutes registers the kiosk page and its API.
func KioskRoutes(router *gin.Engine, h *Handler) {
	k := router.Group("kiosk/:token")
	k.Use(h.kioskAuth)
	{
		k.GET("", h.kioskGetPage)
		k.GET("/state", h.kioskGetState)
		k.POST("/tick", h.kioskTick)
		k.POST("/untick", h.kioskUntick)
		k.POST("/unlock", h.kioskUnlock)
		k.POST("/lock", h.kioskLock)
		k.GET("/pending", h.kioskParentOnly, h.kioskPending)
		k.POST("/approve", h.kioskParentOnly, h.kioskBulk(true))
		k.POST("/reject", h.kioskParentOnly, h.kioskBulk(false))
	}
}
