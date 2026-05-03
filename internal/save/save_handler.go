package save

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/leosu-cela/dog-company/pkg/tool"
)

const (
	MinSupportedVersion = 2
	MaxSupportedVersion = 3
	LogMaxEntries       = 10
	MaxClients          = 30
	MaxBankrupt         = 5
	MaxTutorialStep     = 8
	MaxOfficeLevel      = 4
	MinDogStat          = 1
	MaxDogStat          = 30
	MaxLoanRepayDays    = 80
	MaxStaff            = 100
	MaxTools            = 100
	MaxToolTraits       = 4
	MaxToolBoost        = 5.0
	MaxAchievements     = 50
	MaxCompanyNameLen   = 8
)

var validProjectStatus = map[string]struct{}{
	"offered": {},
	"active":  {},
	"done":    {},
	"failed":  {},
	"late":    {},
}

var validToolGrade = map[string]struct{}{
	"S": {}, "A": {}, "B": {}, "U": {},
}

var validToolCategory = map[string]struct{}{
	"tech": {}, "design": {}, "marketing": {}, "service": {},
}

var validToolCategoryExtra = map[string]struct{}{
	"CEO": {},
}

// SavePayload is the POST /saves request body.
type SavePayload struct {
	Version  int             `json:"version"  binding:"required" example:"2"`
	Revision int             `json:"revision" example:"3"`
	Data     json.RawMessage `json:"data"     binding:"required" swaggertype:"object"`
}

type GetOutput struct {
	Version   int             `json:"version"`
	Revision  int             `json:"revision"`
	UpdatedAt time.Time       `json:"updated_at"`
	Data      json.RawMessage `json:"data" swaggertype:"object"`
}

type UpsertOutput struct {
	Version   int       `json:"version"`
	Revision  int       `json:"revision"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ConflictData struct {
	ServerRevision  int             `json:"server_revision"`
	ServerUpdatedAt time.Time       `json:"server_updated_at"`
	ServerData      json.RawMessage `json:"server_data" swaggertype:"object"`
}

// saveDataForCheck holds the fields we validate (v2 base + v3 extensions).
// Unknown fields are ignored; the raw JSON is stored intact. v1-only fields
// (morale/health/productivityBoost/...) are not declared, matching the spec's
// "ignore deprecated fields" rule. v3-only fields are pointers/slices so a
// missing v2 payload deserializes cleanly without tripping range checks.
type saveDataForCheck struct {
	Day                    int               `json:"day"`
	Money                  int               `json:"money"`
	Reputation             float64           `json:"reputation"`
	TierBudget             int               `json:"tierBudget"`
	OfficeLevel            int               `json:"officeLevel"`
	OfficeSkin             *int              `json:"officeSkin"`
	CompanyName            *string           `json:"companyName"`
	CompanyBuffs           companyBuffs      `json:"companyBuffs"`
	ProjectsCompleted      int               `json:"projectsCompleted"`
	ProjectsFailed         int               `json:"projectsFailed"`
	BankruptCountdown      int               `json:"bankruptCountdown"`
	TutorialStep           int               `json:"tutorialStep"`
	LoanTaken              bool              `json:"loanTaken"`
	LoanRepayDaysLeft      int               `json:"loanRepayDaysLeft"`
	Clients                []project         `json:"clients"`
	Staff                  []dog             `json:"staff"`
	Log                    []json.RawMessage `json:"log"`
	Tools                  []tool_            `json:"tools"`
	UnlockedAchievementIDs []string          `json:"unlockedAchievementIds"`
	ClaimedStarterPack     bool              `json:"claimedStarterPack"`
}

// companyBuffs covers v2 (int) + v3 additions (number + optional sub-objects).
// Boost fields are float64 so v3's fractional buffs (e.g. 0.5) survive the
// round-trip; v2 ints decode into float64 without loss.
type companyBuffs struct {
	SpeedBoost           float64            `json:"speedBoost"`
	QualityBoost         float64            `json:"qualityBoost"`
	TeamworkBoost        float64            `json:"teamworkBoost"`
	CharismaBoost        float64            `json:"charismaBoost"`
	Decor                float64            `json:"decor"`
	CategorySpeed        map[string]float64 `json:"categorySpeed"`
	CategoryQuality      map[string]float64 `json:"categoryQuality"`
	PatienceBoost        float64            `json:"patienceBoost"`
	FatigueRecoveryBonus float64            `json:"fatigueRecoveryBonus"`
}

// dogStats: v2 has 4 dims; v3 adds patience as a 5th. Patience is a pointer
// so missing-from-v2 doesn't trip the [1,10] range check.
type dogStats struct {
	Speed    int  `json:"speed"`
	Quality  int  `json:"quality"`
	Teamwork int  `json:"teamwork"`
	Charisma int  `json:"charisma"`
	Patience *int `json:"patience"`
}

// fatigue/loyalty are floats — game logic applies fractional deltas
// (e.g. loyalty +0.5 per day at company). Stored as number, validated as range.
// Note: v3 dropped Dog.morale entirely; ignore it on inbound (spec says don't
// sanity-check it, since v3 clients never send it).
type dog struct {
	ID               string            `json:"id"`
	IsCEO            bool              `json:"isCEO"`
	Stats            dogStats          `json:"stats"`
	Fatigue          float64           `json:"fatigue"`
	Loyalty          float64           `json:"loyalty"`
	Experience       int               `json:"experience"`
	DaysAtCompany    int               `json:"daysAtCompany"`
	UnhappyLeaveDays int               `json:"unhappyLeaveDays"`
	LearnedTraits    []string          `json:"learnedTraits"`
	RosterID         *string           `json:"rosterId"`
	Level            *int              `json:"level"`
	Fragments        int               `json:"fragments"`
	Status           *string           `json:"status"`
	PipDaysLeft      *int              `json:"pipDaysLeft"`
	PipTasks         []json.RawMessage `json:"pipTasks"`
	EquippedToolID   *string           `json:"equippedToolId"`
}

type project struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

// tool_ avoids colliding with the imported pkg/tool package name.
type tool_ struct {
	InstanceID   string   `json:"instanceId"`
	DefID        string   `json:"defId"`
	Name         string   `json:"name"`
	IconName     string   `json:"iconName"`
	Category     string   `json:"category"`
	Grade        string   `json:"grade"`
	SpeedBoost   float64  `json:"speedBoost"`
	QualityBoost float64  `json:"qualityBoost"`
	Traits       []string `json:"traits"`
	ObtainedDay  int      `json:"obtainedDay"`
}

type SaveHandler struct {
	db   *gorm.DB
	repo ISaveRepository
}

func NewSaveHandler(db *gorm.DB, repo ISaveRepository) *SaveHandler {
	return &SaveHandler{db: db, repo: repo}
}

func (handler *SaveHandler) Get(ctx context.Context, uid uuid.UUID) tool.CommonResponse {
	group := "[SaveHandler@Get]"

	tx := handler.db.WithContext(ctx)
	s, err := handler.repo.FindByUserUID(tx, uid)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return tool.OK(nil)
		}
		log.Printf("%s repo.FindByUserUID failed: %v", group, err)
		return tool.Err(tool.CodeInternal, "internal error")
	}

	return tool.OK(GetOutput{
		Version:   s.Version,
		Revision:  s.Revision,
		UpdatedAt: s.UpdatedAt,
		Data:      s.Data,
	})
}

func (handler *SaveHandler) Upsert(ctx context.Context, uid uuid.UUID, payload SavePayload) tool.CommonResponse {
	group := "[SaveHandler@Upsert]"

	if payload.Version < MinSupportedVersion || payload.Version > MaxSupportedVersion {
		return tool.Err(tool.CodeSaveUnsupportedVersion, fmt.Sprintf("unsupported version %d (server supports %d-%d)", payload.Version, MinSupportedVersion, MaxSupportedVersion))
	}

	var data saveDataForCheck
	if err := json.Unmarshal(payload.Data, &data); err != nil {
		return tool.Err(tool.CodeBadPayload, "data is not valid json")
	}
	if err := sanityCheck(&data); err != nil {
		return tool.Err(tool.CodeSanityFailed, err.Error())
	}

	var out UpsertOutput
	var conflict *ConflictData
	var sanityFailMsg string

	txErr := handler.db.WithContext(ctx).Transaction(func(itx *gorm.DB) error {
		existing, err := handler.repo.FindByUserUIDForUpdate(itx, uid)
		if err != nil && !errors.Is(err, ErrNotFound) {
			return err
		}

		if existing == nil {
			now := time.Now()
			newSave := &Save{
				UserUID:   uid,
				Version:   payload.Version,
				Revision:  1,
				Data:      payload.Data,
				UpdatedAt: now,
			}
			if err := handler.repo.Create(itx, newSave); err != nil {
				return err
			}
			out = UpsertOutput{Version: newSave.Version, Revision: newSave.Revision, UpdatedAt: now}
			return nil
		}

		if existing.Revision != payload.Revision {
			conflict = &ConflictData{
				ServerRevision:  existing.Revision,
				ServerUpdatedAt: existing.UpdatedAt,
				ServerData:      existing.Data,
			}
			return errSaveConflict
		}

		if msg, ok := monotonicCheck(&data, existing.Data); !ok {
			sanityFailMsg = msg
			return errSanityFailed
		}

		now := time.Now()
		newRevision := existing.Revision + 1
		if err := handler.repo.UpdateRevisionAndData(itx, uid, payload.Version, newRevision, payload.Data, now); err != nil {
			return err
		}
		out = UpsertOutput{Version: payload.Version, Revision: newRevision, UpdatedAt: now}
		return nil
	})

	switch {
	case txErr == nil:
		return tool.OK(out)
	case errors.Is(txErr, errSaveConflict):
		return tool.ErrWithData(tool.CodeSaveConflict, "save conflict", conflict)
	case errors.Is(txErr, errSanityFailed):
		return tool.Err(tool.CodeSanityFailed, sanityFailMsg)
	default:
		log.Printf("%s tx failed: %v", group, txErr)
		return tool.Err(tool.CodeInternal, "internal error")
	}
}

func (handler *SaveHandler) Delete(ctx context.Context, uid uuid.UUID) tool.CommonResponse {
	group := "[SaveHandler@Delete]"

	tx := handler.db.WithContext(ctx)
	if err := handler.repo.DeleteByUserUID(tx, uid); err != nil {
		log.Printf("%s repo.DeleteByUserUID failed: %v", group, err)
		return tool.Err(tool.CodeInternal, "internal error")
	}
	return tool.OK(nil)
}

var (
	errSaveConflict = errors.New("save conflict")
	errSanityFailed = errors.New("sanity failed")
)

func sanityCheck(d *saveDataForCheck) error {
	if d.Day < 1 {
		return fmt.Errorf("day must be >= 1 (got %d)", d.Day)
	}
	if d.Money < 0 {
		return fmt.Errorf("money must be >= 0 (got %d)", d.Money)
	}
	if d.Reputation < 0 || d.Reputation > 100 {
		return fmt.Errorf("reputation must be in [0,100] (got %g)", d.Reputation)
	}
	if d.TierBudget < 0 {
		return fmt.Errorf("tierBudget must be >= 0 (got %d)", d.TierBudget)
	}
	if d.OfficeLevel < 0 || d.OfficeLevel > MaxOfficeLevel {
		return fmt.Errorf("officeLevel must be in [0,%d] (got %d)", MaxOfficeLevel, d.OfficeLevel)
	}
	if d.OfficeSkin != nil {
		if *d.OfficeSkin < 0 || *d.OfficeSkin > MaxOfficeLevel {
			return fmt.Errorf("officeSkin must be in [0,%d] (got %d)", MaxOfficeLevel, *d.OfficeSkin)
		}
	}
	if d.CompanyName != nil && len([]rune(*d.CompanyName)) > MaxCompanyNameLen {
		return fmt.Errorf("companyName length must be <= %d runes (got %d)", MaxCompanyNameLen, len([]rune(*d.CompanyName)))
	}
	if d.ProjectsCompleted < 0 {
		return fmt.Errorf("projectsCompleted must be >= 0 (got %d)", d.ProjectsCompleted)
	}
	if d.ProjectsFailed < 0 {
		return fmt.Errorf("projectsFailed must be >= 0 (got %d)", d.ProjectsFailed)
	}
	if d.BankruptCountdown < 0 || d.BankruptCountdown > MaxBankrupt {
		return fmt.Errorf("bankruptCountdown must be in [0,%d] (got %d)", MaxBankrupt, d.BankruptCountdown)
	}
	if d.TutorialStep < 0 || d.TutorialStep > MaxTutorialStep {
		return fmt.Errorf("tutorialStep must be in [0,%d] (got %d)", MaxTutorialStep, d.TutorialStep)
	}
	if d.LoanRepayDaysLeft < 0 || d.LoanRepayDaysLeft > MaxLoanRepayDays {
		return fmt.Errorf("loanRepayDaysLeft must be in [0,%d] (got %d)", MaxLoanRepayDays, d.LoanRepayDaysLeft)
	}
	if err := checkBuffs(&d.CompanyBuffs); err != nil {
		return err
	}
	if len(d.Log) > LogMaxEntries {
		return fmt.Errorf("log must have <= %d entries (got %d)", LogMaxEntries, len(d.Log))
	}
	if len(d.Clients) > MaxClients {
		return fmt.Errorf("clients must have <= %d entries (got %d)", MaxClients, len(d.Clients))
	}
	for i, p := range d.Clients {
		if _, ok := validProjectStatus[p.Status]; !ok {
			return fmt.Errorf("clients[%d].status %q is not a valid enum", i, p.Status)
		}
	}
	if len(d.Staff) > MaxStaff {
		return fmt.Errorf("staff length must be <= %d (got %d)", MaxStaff, len(d.Staff))
	}
	for i, dg := range d.Staff {
		if dg.IsCEO {
			continue
		}
		if err := checkDog(i, &dg); err != nil {
			return err
		}
	}
	if len(d.Tools) > MaxTools {
		return fmt.Errorf("tools length must be <= %d (got %d)", MaxTools, len(d.Tools))
	}
	for i, t := range d.Tools {
		if err := checkTool(i, &t); err != nil {
			return err
		}
	}
	if len(d.UnlockedAchievementIDs) > MaxAchievements {
		return fmt.Errorf("unlockedAchievementIds length must be <= %d (got %d)", MaxAchievements, len(d.UnlockedAchievementIDs))
	}
	return nil
}

func checkTool(i int, t *tool_) error {
	if _, ok := validToolGrade[t.Grade]; !ok {
		return fmt.Errorf("tools[%d].grade %q is not a valid enum", i, t.Grade)
	}
	if _, ok := validToolCategory[t.Category]; !ok {
		if _, ok2 := validToolCategoryExtra[t.Category]; !ok2 {
			return fmt.Errorf("tools[%d].category %q is not a valid enum", i, t.Category)
		}
	}
	if t.SpeedBoost < 0 || t.SpeedBoost > MaxToolBoost {
		return fmt.Errorf("tools[%d].speedBoost must be in [0,%g] (got %g)", i, MaxToolBoost, t.SpeedBoost)
	}
	if t.QualityBoost < 0 || t.QualityBoost > MaxToolBoost {
		return fmt.Errorf("tools[%d].qualityBoost must be in [0,%g] (got %g)", i, MaxToolBoost, t.QualityBoost)
	}
	if len(t.Traits) > MaxToolTraits {
		return fmt.Errorf("tools[%d].traits length must be <= %d (got %d)", i, MaxToolTraits, len(t.Traits))
	}
	return nil
}

func checkBuffs(b *companyBuffs) error {
	for name, v := range map[string]float64{
		"speedBoost":           b.SpeedBoost,
		"qualityBoost":         b.QualityBoost,
		"teamworkBoost":        b.TeamworkBoost,
		"charismaBoost":        b.CharismaBoost,
		"decor":                b.Decor,
		"patienceBoost":        b.PatienceBoost,
		"fatigueRecoveryBonus": b.FatigueRecoveryBonus,
	} {
		if v < 0 {
			return fmt.Errorf("companyBuffs.%s must be >= 0 (got %g)", name, v)
		}
	}
	for k, v := range b.CategorySpeed {
		if _, ok := validToolCategory[k]; !ok {
			return fmt.Errorf("companyBuffs.categorySpeed has invalid category %q", k)
		}
		if v < 0 {
			return fmt.Errorf("companyBuffs.categorySpeed[%s] must be >= 0 (got %g)", k, v)
		}
	}
	for k, v := range b.CategoryQuality {
		if _, ok := validToolCategory[k]; !ok {
			return fmt.Errorf("companyBuffs.categoryQuality has invalid category %q", k)
		}
		if v < 0 {
			return fmt.Errorf("companyBuffs.categoryQuality[%s] must be >= 0 (got %g)", k, v)
		}
	}
	return nil
}

func checkDog(i int, dg *dog) error {
	stats := map[string]int{
		"speed":    dg.Stats.Speed,
		"quality":  dg.Stats.Quality,
		"teamwork": dg.Stats.Teamwork,
		"charisma": dg.Stats.Charisma,
	}
	if dg.Stats.Patience != nil {
		stats["patience"] = *dg.Stats.Patience
	}
	for name, v := range stats {
		if v < MinDogStat || v > MaxDogStat {
			return fmt.Errorf("staff[%d].stats.%s must be in [%d,%d] (got %d)", i, name, MinDogStat, MaxDogStat, v)
		}
	}
	for name, v := range map[string]float64{
		"fatigue": dg.Fatigue,
		"loyalty": dg.Loyalty,
	} {
		if v < 0 || v > 100 {
			return fmt.Errorf("staff[%d].%s must be in [0,100] (got %g)", i, name, v)
		}
	}
	for name, v := range map[string]int{
		"experience":       dg.Experience,
		"daysAtCompany":    dg.DaysAtCompany,
		"unhappyLeaveDays": dg.UnhappyLeaveDays,
		"fragments":        dg.Fragments,
	} {
		if v < 0 {
			return fmt.Errorf("staff[%d].%s must be >= 0 (got %d)", i, name, v)
		}
	}
	if dg.Level != nil && (*dg.Level < 1 || *dg.Level > 10) {
		return fmt.Errorf("staff[%d].level must be in [1,10] (got %d)", i, *dg.Level)
	}
	if len(dg.LearnedTraits) > 8 {
		return fmt.Errorf("staff[%d].learnedTraits length must be <= 8 (got %d)", i, len(dg.LearnedTraits))
	}
	if dg.Status != nil && *dg.Status != "active" && *dg.Status != "pip" {
		return fmt.Errorf("staff[%d].status %q must be active or pip", i, *dg.Status)
	}
	if dg.PipDaysLeft != nil && *dg.PipDaysLeft < 0 {
		return fmt.Errorf("staff[%d].pipDaysLeft must be >= 0 (got %d)", i, *dg.PipDaysLeft)
	}
	if len(dg.PipTasks) > 10 {
		return fmt.Errorf("staff[%d].pipTasks length must be <= 10 (got %d)", i, len(dg.PipTasks))
	}
	if dg.RosterID != nil {
		if _, ok := validRosterID[*dg.RosterID]; !ok {
			return fmt.Errorf("staff[%d].rosterId %q not in roster whitelist", i, *dg.RosterID)
		}
	}
	return nil
}

// monotonicCheckFields is the minimal projection of the saved JSON we need
// for monotonic comparison. Unmarshaling only these two fields skips the
// expensive Clients/Staff/Log array decoding on every Upsert.
type monotonicCheckFields struct {
	Day               int `json:"day"`
	ProjectsCompleted int `json:"projectsCompleted"`
}

func monotonicCheck(d *saveDataForCheck, prevRaw []byte) (string, bool) {
	var prev monotonicCheckFields
	if err := json.Unmarshal(prevRaw, &prev); err != nil {
		return "", true
	}
	if d.Day < prev.Day {
		return fmt.Sprintf("day cannot decrease (new=%d, prev=%d)", d.Day, prev.Day), false
	}
	if d.ProjectsCompleted < prev.ProjectsCompleted {
		return fmt.Sprintf("projectsCompleted cannot decrease (new=%d, prev=%d)", d.ProjectsCompleted, prev.ProjectsCompleted), false
	}
	return "", true
}
