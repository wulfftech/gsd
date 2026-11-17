// Package household holds the per-circle settings that sit beside chores:
// school-holiday periods and the shared kiosk's credentials. It depends only on
// the database so the chore and notifier packages can both use it.
package household

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// DateLayout is how holiday dates travel over the API: a local calendar date.
const DateLayout = "2006-01-02"

// Holiday is an inclusive range of local calendar dates.
type Holiday struct {
	ID        int       `json:"id" gorm:"primary_key"`
	CircleID  int       `json:"-" gorm:"column:circle_id;index"`
	Name      string    `json:"name" gorm:"column:name"`
	StartDate string    `json:"startDate" gorm:"column:start_date"`
	EndDate   string    `json:"endDate" gorm:"column:end_date"`
	CreatedAt time.Time `json:"-" gorm:"column:created_at"`
}

// Settings holds the kiosk credentials for one circle. Both secrets are stored
// hashed: the token with SHA-256 (it is long and random), the PIN with bcrypt.
type Settings struct {
	CircleID       int       `gorm:"column:circle_id;primaryKey;autoIncrement:false"`
	KioskTokenHash string    `gorm:"column:kiosk_token_hash;index"`
	ParentPinHash  string    `gorm:"column:parent_pin_hash"`
	UpdatedAt      time.Time `gorm:"column:updated_at"`
}

func (Settings) TableName() string { return "household_settings" }

// Range is a holiday resolved to local midnights, End exclusive.
type Range struct {
	Start time.Time
	End   time.Time
}

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) ListHolidays(ctx context.Context, circleID int) ([]*Holiday, error) {
	var hs []*Holiday
	err := r.db.WithContext(ctx).Where("circle_id = ?", circleID).Order("start_date").Find(&hs).Error
	return hs, err
}

// ValidateHoliday checks both dates parse and are in order.
func ValidateHoliday(h *Holiday) error {
	s, err := time.Parse(DateLayout, h.StartDate)
	if err != nil {
		return fmt.Errorf("startDate must be YYYY-MM-DD")
	}
	e, err := time.Parse(DateLayout, h.EndDate)
	if err != nil {
		return fmt.Errorf("endDate must be YYYY-MM-DD")
	}
	if e.Before(s) {
		return fmt.Errorf("endDate is before startDate")
	}
	if h.Name == "" {
		return fmt.Errorf("name is required")
	}
	return nil
}

func (r *Repository) CreateHoliday(ctx context.Context, h *Holiday) error {
	if err := ValidateHoliday(h); err != nil {
		return err
	}
	h.CreatedAt = time.Now().UTC()
	return r.db.WithContext(ctx).Create(h).Error
}

func (r *Repository) DeleteHoliday(ctx context.Context, circleID, id int) error {
	res := r.db.WithContext(ctx).Where("circle_id = ? AND id = ?", circleID, id).Delete(&Holiday{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// HolidayRanges returns the circle's holidays as [start, end) instants in loc.
func (r *Repository) HolidayRanges(ctx context.Context, circleID int, loc *time.Location) ([]Range, error) {
	hs, err := r.ListHolidays(ctx, circleID)
	if err != nil {
		return nil, err
	}
	return ToRanges(hs, loc), nil
}

func ToRanges(hs []*Holiday, loc *time.Location) []Range {
	out := make([]Range, 0, len(hs))
	for _, h := range hs {
		s, err1 := time.ParseInLocation(DateLayout, h.StartDate, loc)
		e, err2 := time.ParseInLocation(DateLayout, h.EndDate, loc)
		if err1 != nil || err2 != nil {
			continue
		}
		out = append(out, Range{Start: s, End: e.AddDate(0, 0, 1)})
	}
	return out
}

func (r *Repository) getSettings(ctx context.Context, circleID int) (*Settings, error) {
	var s Settings
	err := r.db.WithContext(ctx).Where("circle_id = ?", circleID).First(&s).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &Settings{CircleID: circleID}, nil
	}
	return &s, err
}

func (r *Repository) saveSettings(ctx context.Context, s *Settings) error {
	s.UpdatedAt = time.Now().UTC()
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{UpdateAll: true}).Create(s).Error
}

// KioskStatus reports whether a kiosk link and a parent PIN have been set.
func (r *Repository) KioskStatus(ctx context.Context, circleID int) (tokenSet, pinSet bool, err error) {
	s, err := r.getSettings(ctx, circleID)
	if err != nil {
		return false, false, err
	}
	return s.KioskTokenHash != "", s.ParentPinHash != "", nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// NewKioskToken replaces the circle's kiosk token and returns the new one.
func (r *Repository) NewKioskToken(ctx context.Context, circleID int) (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := hex.EncodeToString(b)
	s, err := r.getSettings(ctx, circleID)
	if err != nil {
		return "", err
	}
	s.KioskTokenHash = hashToken(token)
	return token, r.saveSettings(ctx, s)
}

// CircleForKioskToken resolves a kiosk token to its circle.
func (r *Repository) CircleForKioskToken(ctx context.Context, token string) (int, error) {
	if len(token) < 32 {
		return 0, gorm.ErrRecordNotFound
	}
	h := hashToken(token)
	var s Settings
	if err := r.db.WithContext(ctx).Where("kiosk_token_hash = ?", h).First(&s).Error; err != nil {
		return 0, err
	}
	if subtle.ConstantTimeCompare([]byte(s.KioskTokenHash), []byte(h)) != 1 {
		return 0, gorm.ErrRecordNotFound
	}
	return s.CircleID, nil
}

// ValidPIN is 4 to 8 digits.
func ValidPIN(pin string) bool {
	if len(pin) < 4 || len(pin) > 8 {
		return false
	}
	for _, c := range pin {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func (r *Repository) SetParentPIN(ctx context.Context, circleID int, pin string) error {
	if !ValidPIN(pin) {
		return fmt.Errorf("PIN must be 4 to 8 digits")
	}
	h, err := bcrypt.GenerateFromPassword([]byte(pin), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	s, err := r.getSettings(ctx, circleID)
	if err != nil {
		return err
	}
	s.ParentPinHash = string(h)
	return r.saveSettings(ctx, s)
}

// CheckParentPIN reports whether pin matches. False when no PIN is set.
func (r *Repository) CheckParentPIN(ctx context.Context, circleID int, pin string) (bool, error) {
	s, err := r.getSettings(ctx, circleID)
	if err != nil {
		return false, err
	}
	if s.ParentPinHash == "" {
		return false, nil
	}
	return bcrypt.CompareHashAndPassword([]byte(s.ParentPinHash), []byte(pin)) == nil, nil
}
