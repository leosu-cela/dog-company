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
	SupportedVersion = 1
	LogMaxEntries    = 10
)

// SavePayload is the POST /saves request body.
type SavePayload struct {
	Version  int             `json:"version"  binding:"required" example:"1"`
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

// saveDataForCheck holds only the fields needed for sanity validation.
// Unknown fields are ignored; the raw JSON is stored intact.
type saveDataForCheck struct {
	Day               int               `json:"day"`
	Money             int               `json:"money"`
	Morale            int               `json:"morale"`
	Health            int               `json:"health"`
	OfficeLevel       int               `json:"officeLevel"`
	ProductivityBoost int               `json:"productivityBoost"`
	StabilityBoost    int               `json:"stabilityBoost"`
	TrainingBoost     int               `json:"trainingBoost"`
	TutorialStep     int                `json:"tutorialStep"`
	Log               []json.RawMessage `json:"log"`
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
	errSaveConflict  = errors.New("save conflict")
	errSanityFailed  = errors.New("sanity failed")
)

func sanityCheck(d *saveDataForCheck) error {
	if d.Day < 1 {
		return fmt.Errorf("day must be >= 1 (got %d)", d.Day)
	}
	if d.Money < 0 {
		return fmt.Errorf("money must be >= 0 (got %d)", d.Money)
	}
	if d.Morale < 0 || d.Morale > 100 {
		return fmt.Errorf("morale must be in [0,100] (got %d)", d.Morale)
	}
	if d.Health < 0 || d.Health > 100 {
		return fmt.Errorf("health must be in [0,100] (got %d)", d.Health)
	}
	if d.OfficeLevel < 0 {
		return fmt.Errorf("officeLevel must be >= 0 (got %d)", d.OfficeLevel)
	}
	if d.ProductivityBoost < 0 {
		return fmt.Errorf("productivityBoost must be >= 0 (got %d)", d.ProductivityBoost)
	}
	if d.StabilityBoost < 0 {
		return fmt.Errorf("stabilityBoost must be >= 0 (got %d)", d.StabilityBoost)
	}
	if d.TrainingBoost < 0 {
		return fmt.Errorf("trainingBoost must be >= 0 (got %d)", d.TrainingBoost)
	}
	if d.TutorialStep < 0 || d.TutorialStep > 7 {
		return fmt.Errorf("tutorialStep must be in [0,7] (got %d)", d.TutorialStep)
	}
	if len(d.Log) > LogMaxEntries {
		return fmt.Errorf("log must have <= %d entries (got %d)", LogMaxEntries, len(d.Log))
	}
	return nil
}

func monotonicCheck(d *saveDataForCheck, prevRaw []byte) (string, bool) {
	var prev saveDataForCheck
	if err := json.Unmarshal(prevRaw, &prev); err != nil {
		return "", true
	}
	if d.Day < prev.Day {
		return fmt.Sprintf("day cannot decrease (new=%d, prev=%d)", d.Day, prev.Day), false
	}
	return "", true
}
