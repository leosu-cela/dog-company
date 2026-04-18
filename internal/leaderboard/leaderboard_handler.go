package leaderboard

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/leosu-cela/dog-company/internal/user"
	"github.com/leosu-cela/dog-company/pkg/tool"
)

const (
	DefaultGoal      = 50000
	DefaultLimit     = 10
	DefaultMineLimit = 20
	MaxLimit         = 50
	MaxDays          = 365
	MaxOfficeLevel   = 4
	MaxStaffCount    = 50
	DedupeWindow     = time.Minute
	MoneyMultiplier  = 3
	ListCacheTTL     = 30 * time.Minute
)

var allowedGoals = map[int]struct{}{50000: {}}

type SubmitPayload struct {
	Days        int `json:"days"         binding:"required" example:"58"`
	Money       int `json:"money"        binding:"required" example:"52340"`
	Goal        int `json:"goal"         binding:"required" example:"50000"`
	OfficeLevel int `json:"office_level" example:"4"`
	StaffCount  int `json:"staff_count"  example:"9"`
}

type EntryOutput struct {
	ID          uint64    `json:"id"`
	UserID      uint64    `json:"user_id"`
	Nickname    string    `json:"nickname"`
	Days        int       `json:"days"`
	Money       int       `json:"money"`
	Goal        int       `json:"goal"`
	OfficeLevel int       `json:"office_level"`
	StaffCount  int       `json:"staff_count"`
	SubmittedAt time.Time `json:"submitted_at"`
}

type MyBest struct {
	Rank  int         `json:"rank"`
	Entry EntryOutput `json:"entry"`
}

type ListOutput struct {
	Entries []EntryOutput `json:"entries"`
	MyBest  *MyBest       `json:"my_best,omitempty"`
}

type ListMineOutput struct {
	Entries []EntryOutput `json:"entries"`
}

type SubmitOutput struct {
	ID    uint64 `json:"id"`
	Rank  int    `json:"rank"`
	Total int    `json:"total"`
}

type ListInput struct {
	Goal  int
	Limit int
	UID   *uuid.UUID
}

type ListMineInput struct {
	UID   uuid.UUID
	Goal  int
	Limit int
}

type LeaderboardHandler struct {
	db        *gorm.DB
	repo      IEntryRepository
	userRepo  user.IUserRepository
	listCache *ListCache
}

func NewLeaderboardHandler(db *gorm.DB, repo IEntryRepository, userRepo user.IUserRepository, listCache *ListCache) *LeaderboardHandler {
	return &LeaderboardHandler{db: db, repo: repo, userRepo: userRepo, listCache: listCache}
}

func (handler *LeaderboardHandler) List(ctx context.Context, in ListInput) tool.CommonResponse {
	group := "[LeaderboardHandler@List]"

	goal := in.Goal
	if goal == 0 {
		goal = DefaultGoal
	}
	limit := clampLimit(in.Limit, DefaultLimit, MaxLimit)

	tx := handler.db.WithContext(ctx)

	entries, cached := handler.listCache.Get(goal, limit)
	if !cached {
		fresh, err := handler.repo.List(tx, goal, limit)
		if err != nil {
			log.Printf("%s repo.List failed: %v", group, err)
			return tool.Err(tool.CodeInternal, "internal error")
		}
		handler.listCache.Set(goal, limit, fresh)
		entries = fresh
	}

	out := ListOutput{Entries: make([]EntryOutput, 0, len(entries))}
	for _, e := range entries {
		out.Entries = append(out.Entries, toEntryOutput(&e))
	}

	if in.UID != nil {
		out.MyBest = handler.tryMyBest(tx, *in.UID, goal, group)
	}

	return tool.OK(out)
}

// tryMyBest returns the user's best entry + rank for the goal.
// Any lookup failure is logged and returns nil — the my_best field is optional
// and must not break the public list.
func (handler *LeaderboardHandler) tryMyBest(tx *gorm.DB, uid uuid.UUID, goal int, group string) *MyBest {
	u, err := handler.userRepo.FindByUID(tx, uid)
	if err != nil {
		if !errors.Is(err, user.ErrNotFound) {
			log.Printf("%s tryMyBest userRepo.FindByUID failed: %v", group, err)
		}
		return nil
	}
	best, err := handler.repo.FindBestByUserAndGoal(tx, u.ID, goal)
	if err != nil {
		if !errors.Is(err, ErrNotFound) {
			log.Printf("%s tryMyBest repo.FindBestByUserAndGoal failed: %v", group, err)
		}
		return nil
	}
	better, err := handler.repo.CountBetter(tx, best.Goal, best.Days, best.Money)
	if err != nil {
		log.Printf("%s tryMyBest repo.CountBetter failed: %v", group, err)
		return nil
	}
	return &MyBest{
		Rank:  int(better) + 1,
		Entry: toEntryOutput(best),
	}
}

func (handler *LeaderboardHandler) ListMine(ctx context.Context, in ListMineInput) tool.CommonResponse {
	group := "[LeaderboardHandler@ListMine]"

	limit := clampLimit(in.Limit, DefaultMineLimit, MaxLimit)
	tx := handler.db.WithContext(ctx)

	u, err := handler.userRepo.FindByUID(tx, in.UID)
	if err != nil {
		if errors.Is(err, user.ErrNotFound) {
			return tool.Err(tool.CodeUnauthorized, "user not found")
		}
		log.Printf("%s userRepo.FindByUID failed: %v", group, err)
		return tool.Err(tool.CodeInternal, "internal error")
	}

	entries, err := handler.repo.ListByUser(tx, u.ID, in.Goal, limit)
	if err != nil {
		log.Printf("%s repo.ListByUser failed: %v", group, err)
		return tool.Err(tool.CodeInternal, "internal error")
	}

	out := ListMineOutput{Entries: make([]EntryOutput, 0, len(entries))}
	for _, e := range entries {
		out.Entries = append(out.Entries, toEntryOutput(&e))
	}
	return tool.OK(out)
}

func (handler *LeaderboardHandler) Submit(ctx context.Context, uid uuid.UUID, payload SubmitPayload) tool.CommonResponse {
	group := "[LeaderboardHandler@Submit]"

	if err := sanityCheck(&payload); err != nil {
		return tool.Err(tool.CodeSanityFailed, err.Error())
	}

	tx := handler.db.WithContext(ctx)

	u, err := handler.userRepo.FindByUID(tx, uid)
	if err != nil {
		if errors.Is(err, user.ErrNotFound) {
			return tool.Err(tool.CodeUnauthorized, "user not found")
		}
		log.Printf("%s userRepo.FindByUID failed: %v", group, err)
		return tool.Err(tool.CodeInternal, "internal error")
	}

	var out SubmitOutput

	txErr := tx.Transaction(func(itx *gorm.DB) error {
		dup, err := handler.repo.FindRecentDuplicate(itx, u.ID, payload.Goal, payload.Days, payload.Money, DedupeWindow)
		if err != nil && !errors.Is(err, ErrNotFound) {
			return err
		}

		var target *Entry
		if dup != nil {
			target = dup
		} else {
			e := &Entry{
				UserID:      u.ID,
				Nickname:    u.Account,
				Days:        payload.Days,
				Money:       payload.Money,
				Goal:        payload.Goal,
				OfficeLevel: payload.OfficeLevel,
				StaffCount:  payload.StaffCount,
				SubmittedAt: time.Now(),
			}
			if err := handler.repo.Create(itx, e); err != nil {
				return err
			}
			target = e
		}

		better, err := handler.repo.CountBetter(itx, target.Goal, target.Days, target.Money)
		if err != nil {
			return err
		}
		total, err := handler.repo.CountByGoal(itx, target.Goal)
		if err != nil {
			return err
		}

		out = SubmitOutput{
			ID:    target.ID,
			Rank:  int(better) + 1,
			Total: int(total),
		}
		return nil
	})

	if txErr != nil {
		log.Printf("%s tx failed: %v", group, txErr)
		return tool.Err(tool.CodeInternal, "internal error")
	}

	handler.listCache.Invalidate()

	return tool.OK(out)
}

func clampLimit(raw, def, max int) int {
	if raw <= 0 {
		return def
	}
	if raw > max {
		return max
	}
	return raw
}

func toEntryOutput(e *Entry) EntryOutput {
	return EntryOutput{
		ID:          e.ID,
		UserID:      e.UserID,
		Nickname:    e.Nickname,
		Days:        e.Days,
		Money:       e.Money,
		Goal:        e.Goal,
		OfficeLevel: e.OfficeLevel,
		StaffCount:  e.StaffCount,
		SubmittedAt: e.SubmittedAt,
	}
}

func sanityCheck(p *SubmitPayload) error {
	if p.Days < 1 || p.Days > MaxDays {
		return fmt.Errorf("days must be in [1,%d] (got %d)", MaxDays, p.Days)
	}
	if _, ok := allowedGoals[p.Goal]; !ok {
		return fmt.Errorf("goal %d is not supported", p.Goal)
	}
	if p.Money < p.Goal {
		return fmt.Errorf("money must be >= goal (money=%d, goal=%d)", p.Money, p.Goal)
	}
	if p.Money > p.Goal*MoneyMultiplier {
		return fmt.Errorf("money suspiciously high (money=%d, max=%d)", p.Money, p.Goal*MoneyMultiplier)
	}
	if p.OfficeLevel < 0 || p.OfficeLevel > MaxOfficeLevel {
		return fmt.Errorf("office_level must be in [0,%d] (got %d)", MaxOfficeLevel, p.OfficeLevel)
	}
	if p.StaffCount < 0 || p.StaffCount > MaxStaffCount {
		return fmt.Errorf("staff_count must be in [0,%d] (got %d)", MaxStaffCount, p.StaffCount)
	}
	return nil
}
