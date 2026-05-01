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
	SupportedVersion = 2
	LogMaxEntries    = 10
	MaxClients       = 30
	MaxBankrupt      = 5
	MaxTutorialStep   = 7
	MaxOfficeLevel    = 4
	MinDogStat        = 1
	MaxDogStat        = 30
	MaxLoanRepayDays  = 80
	MaxStaff          = 100
)

var validProjectStatus = map[string]struct{}{
	"offered": {},
	"active":  {},
	"done":    {},
	"failed":  {},
	"late":    {},
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

// saveDataForCheck holds the v2 fields we validate. Unknown fields are
// ignored; the raw JSON is stored intact. v1-only fields (morale/health/
// productivityBoost/...) are not declared here, matching the spec's
// "ignore deprecated fields" rule.
type saveDataForCheck struct {
	Day               int          `json:"day"`
	Money             int          `json:"money"`
	Reputation        float64      `json:"reputation"`
	TierBudget        int          `json:"tierBudget"`
	OfficeLevel       int          `json:"officeLevel"`
	CompanyBuffs      companyBuffs `json:"companyBuffs"`
	ProjectsCompleted int          `json:"projectsCompleted"`
	ProjectsFailed    int          `json:"projectsFailed"`
	BankruptCountdown int          `json:"bankruptCountdown"`
	TutorialStep      int          `json:"tutorialStep"`
	LoanTaken         bool         `json:"loanTaken"`
	LoanRepayDaysLeft int          `json:"loanRepayDaysLeft"`
	Clients           []project    `json:"clients"`
	Staff             []dog        `json:"staff"`
	Log               []json.RawMessage `json:"log"`
}

type companyBuffs struct {
	SpeedBoost    int `json:"speedBoost"`
	QualityBoost  int `json:"qualityBoost"`
	TeamworkBoost int `json:"teamworkBoost"`
	CharismaBoost int `json:"charismaBoost"`
	Decor         int `json:"decor"`
}

type dogStats struct {
	Speed    int `json:"speed"`
	Quality  int `json:"quality"`
	Teamwork int `json:"teamwork"`
	Charisma int `json:"charisma"`
}

// morale/fatigue/loyalty are floats — game logic applies fractional deltas
// (e.g. loyalty +0.5 per day at company). Stored as number, validated as range.
type dog struct {
	ID      string   `json:"id"`
	IsCEO   bool     `json:"isCEO"`
	Stats   dogStats `json:"stats"`
	Morale  float64  `json:"morale"`
	Fatigue float64  `json:"fatigue"`
	Loyalty float64  `json:"loyalty"`
}

type project struct {
	ID     string `json:"id"`
	Status string `json:"status"`
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

	if payload.Version != SupportedVersion {
		return tool.Err(tool.CodeSaveUnsupportedVersion, fmt.Sprintf("unsupported version %d (server supports %d)", payload.Version, SupportedVersion))
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
	return nil
}

func checkBuffs(b *companyBuffs) error {
	if b.SpeedBoost < 0 {
		return fmt.Errorf("companyBuffs.speedBoost must be >= 0 (got %d)", b.SpeedBoost)
	}
	if b.QualityBoost < 0 {
		return fmt.Errorf("companyBuffs.qualityBoost must be >= 0 (got %d)", b.QualityBoost)
	}
	if b.TeamworkBoost < 0 {
		return fmt.Errorf("companyBuffs.teamworkBoost must be >= 0 (got %d)", b.TeamworkBoost)
	}
	if b.CharismaBoost < 0 {
		return fmt.Errorf("companyBuffs.charismaBoost must be >= 0 (got %d)", b.CharismaBoost)
	}
	if b.Decor < 0 {
		return fmt.Errorf("companyBuffs.decor must be >= 0 (got %d)", b.Decor)
	}
	return nil
}

func checkDog(i int, dg *dog) error {
	for name, v := range map[string]int{
		"speed":    dg.Stats.Speed,
		"quality":  dg.Stats.Quality,
		"teamwork": dg.Stats.Teamwork,
		"charisma": dg.Stats.Charisma,
	} {
		if v < MinDogStat || v > MaxDogStat {
			return fmt.Errorf("staff[%d].stats.%s must be in [%d,%d] (got %d)", i, name, MinDogStat, MaxDogStat, v)
		}
	}
	for name, v := range map[string]float64{
		"morale":  dg.Morale,
		"fatigue": dg.Fatigue,
		"loyalty": dg.Loyalty,
	} {
		if v < 0 || v > 100 {
			return fmt.Errorf("staff[%d].%s must be in [0,100] (got %g)", i, name, v)
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
