package leaderboard

import (
	"context"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/leosu-cela/dog-company/internal/events"
	"github.com/leosu-cela/dog-company/internal/user"
	"github.com/leosu-cela/dog-company/pkg/tool"
)

const (
	DefaultGoal     = 50000
	DefaultLimit    = 10
	MaxLimit        = 50
	MaxDays         = 365 * 5
	MaxOfficeLevel  = 4
	MaxStaffCount   = 100
	MaxProjects     = 365 * 3
	MoneyMultiplier = 5
	MoneyPerDayCap  = 2000
	ListCacheTTL    = 30 * time.Minute

	CompanyNameMinRunes = 2
	CompanyNameMaxRunes = 8

	// 真實 wall-clock 時間下界：每個遊戲內天數至少需要的秒數。
	// 校準依據：遊戲最快 1 天 = 5 秒（含買加速），用 4.9 留誤差緩衝。
	MinSecondsPerDay = 4.9
)

var allowedGoals = map[int]struct{}{50000: {}}

// 白名單：CJK Unified Ideographs (基本平面 + 擴展) + 英數 + ASCII 空白。
// 與前端 src/lib/companyName.ts 對齊。
var companyNameRe = regexp.MustCompile(`^[\p{Han}A-Za-z0-9 ]+$`)

type SubmitPayload struct {
	Days              int    `json:"days"               binding:"required" example:"58"`
	Money             int    `json:"money"              binding:"required" example:"52340"`
	Goal              int    `json:"goal"               example:"50000"`
	OfficeLevel       int    `json:"office_level"       example:"4"`
	StaffCount        int    `json:"staff_count"        example:"9"`
	ProjectsCompleted int    `json:"projects_completed" example:"32"`
	CompanyName       string `json:"company_name"       example:"旺財事務所"`
}

type EntryOutput struct {
	ID                uint64    `json:"id"`
	UserID            uint64    `json:"user_id"`
	Nickname          string    `json:"nickname"`     // 保留欄位（=帳號）；新版前端不再使用
	CompanyName       string    `json:"company_name"` // 玩家自訂公司名（v6 起；舊資料為空字串）
	Days              int       `json:"days"`
	Money             int       `json:"money"`
	Goal              int       `json:"goal"`
	OfficeLevel       int       `json:"office_level"`
	StaffCount        int       `json:"staff_count"`
	ProjectsCompleted int       `json:"projects_completed"`
	SubmittedAt       time.Time `json:"submitted_at"`
}

type Me struct {
	Rank  int         `json:"rank"`
	Entry EntryOutput `json:"entry"`
}

type ListOutput struct {
	Entries []EntryOutput `json:"entries"`
	Me      *Me           `json:"me,omitempty"`
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

type LeaderboardHandler struct {
	db          *gorm.DB
	repo        IEntryRepository
	runRepo     IRunRepository
	userRepo    user.IUserRepository
	listCache   *ListCache
	eventBuffer *events.Buffer
}

func NewLeaderboardHandler(db *gorm.DB, repo IEntryRepository, runRepo IRunRepository, userRepo user.IUserRepository, listCache *ListCache, eventBuffer *events.Buffer) *LeaderboardHandler {
	return &LeaderboardHandler{
		db:          db,
		repo:        repo,
		runRepo:     runRepo,
		userRepo:    userRepo,
		listCache:   listCache,
		eventBuffer: eventBuffer,
	}
}

type StartRunPayload struct {
	Goal int `json:"goal" example:"50000"`
}

type StartRunOutput struct {
	StartedAt time.Time `json:"started_at"`
}

func (handler *LeaderboardHandler) StartRun(ctx context.Context, uid uuid.UUID, payload StartRunPayload) tool.CommonResponse {
	group := "[LeaderboardHandler@StartRun]"

	goal := payload.Goal
	if goal == 0 {
		goal = DefaultGoal
	}
	if _, ok := allowedGoals[goal]; !ok {
		return tool.Err(tool.CodeSanityFailed, fmt.Sprintf("goal %d is not supported", goal))
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

	run, err := handler.runRepo.Upsert(tx, u.ID, goal)
	if err != nil {
		log.Printf("%s runRepo.Upsert failed: %v", group, err)
		return tool.Err(tool.CodeInternal, "internal error")
	}

	events.LogInternal(handler.eventBuffer, u.UID, events.TypeStartRun, map[string]any{
		"goal": goal,
	})

	return tool.OK(StartRunOutput{StartedAt: run.StartedAt})
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
		out.Me = handler.tryMe(tx, *in.UID, goal, group)
	}

	return tool.OK(out)
}

// tryMe returns the user's best entry + global rank for the goal.
// Any lookup failure is logged and returns nil — the me field is optional
// and must not break the public list.
func (handler *LeaderboardHandler) tryMe(tx *gorm.DB, uid uuid.UUID, goal int, group string) *Me {
	u, err := handler.userRepo.FindByUID(tx, uid)
	if err != nil {
		if !errors.Is(err, user.ErrNotFound) {
			log.Printf("%s tryMe userRepo.FindByUID failed: %v", group, err)
		}
		return nil
	}
	best, err := handler.repo.FindBestByUserAndGoal(tx, u.ID, goal)
	if err != nil {
		if !errors.Is(err, ErrNotFound) {
			log.Printf("%s tryMe repo.FindBestByUserAndGoal failed: %v", group, err)
		}
		return nil
	}
	better, err := handler.repo.CountBetter(tx, best.Goal, best.Days, best.Money, best.ProjectsCompleted)
	if err != nil {
		log.Printf("%s tryMe repo.CountBetter failed: %v", group, err)
		return nil
	}
	return &Me{
		Rank:  int(better) + 1,
		Entry: toEntryOutput(best),
	}
}

// Sentinel errors for surfacing user-facing rejection from inside tx.Transaction.
var (
	errNoActiveRun = errors.New("no active run")
	errRunTooShort = errors.New("run too short")
)

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
	var minSec float64

	txErr := tx.Transaction(func(itx *gorm.DB) error {
		// 驗證 run 真實時長下界。
		// 沒有 active run（玩家未呼叫 StartRun，或 token 已被前次提交刪除）→ 拒收。
		// 真實 wall-clock 時間 < days * MinSecondsPerDay → 拒收。
		run, runErr := handler.runRepo.FindByUserAndGoalForUpdate(itx, u.ID, payload.Goal)
		if runErr != nil {
			if errors.Is(runErr, ErrRunNotFound) {
				return errNoActiveRun
			}
			return runErr
		}
		elapsed := time.Since(run.StartedAt).Seconds()
		minSec = float64(payload.Days) * MinSecondsPerDay
		if elapsed < minSec {
			return errRunTooShort
		}

		existing, err := handler.repo.FindByUserAndGoalForUpdate(itx, u.ID, payload.Goal)
		if err != nil && !errors.Is(err, ErrNotFound) {
			return err
		}

		companyName := strings.TrimSpace(payload.CompanyName)
		now := time.Now()

		var target *Entry
		switch {
		case existing == nil:
			e := &Entry{
				UserID:            u.ID,
				Nickname:          u.Account,
				CompanyName:       companyName,
				Days:              payload.Days,
				Money:             payload.Money,
				Goal:              payload.Goal,
				OfficeLevel:       payload.OfficeLevel,
				StaffCount:        payload.StaffCount,
				ProjectsCompleted: payload.ProjectsCompleted,
				SubmittedAt:       now,
			}
			if err := handler.repo.Create(itx, e); err != nil {
				return err
			}
			target = e
		case isBetter(payload.Days, payload.Money, payload.ProjectsCompleted, existing.Days, existing.Money, existing.ProjectsCompleted):
			if err := handler.repo.UpdateBestFields(itx, existing.ID, payload.Days, payload.Money, payload.OfficeLevel, payload.StaffCount, payload.ProjectsCompleted, companyName, now); err != nil {
				return err
			}
			existing.Days = payload.Days
			existing.Money = payload.Money
			existing.OfficeLevel = payload.OfficeLevel
			existing.StaffCount = payload.StaffCount
			existing.ProjectsCompleted = payload.ProjectsCompleted
			existing.CompanyName = companyName
			existing.SubmittedAt = now
			target = existing
		default:
			target = existing
		}

		better, err := handler.repo.CountBetter(itx, target.Goal, target.Days, target.Money, target.ProjectsCompleted)
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

		// 寫入成功 → 同 tx 內刪除 run，避免同筆 run 重送或事後再被竄改起算時間。
		if err := handler.runRepo.DeleteByUserAndGoal(itx, u.ID, payload.Goal); err != nil {
			return err
		}
		return nil
	})

	if errors.Is(txErr, errNoActiveRun) {
		return tool.Err(tool.CodeSanityFailed, "no active run; please start a new game first")
	}
	if errors.Is(txErr, errRunTooShort) {
		return tool.Err(tool.CodeSanityFailed, fmt.Sprintf("run duration too short (min %.1fs)", minSec))
	}
	if txErr != nil {
		log.Printf("%s tx failed: %v", group, txErr)
		return tool.Err(tool.CodeInternal, "internal error")
	}

	handler.listCache.InvalidateGoal(payload.Goal)

	events.LogInternal(handler.eventBuffer, u.UID, events.TypeSubmitLeaderboard, map[string]any{
		"goal":               payload.Goal,
		"days":               payload.Days,
		"money":              payload.Money,
		"office_level":       payload.OfficeLevel,
		"staff_count":        payload.StaffCount,
		"projects_completed": payload.ProjectsCompleted,
	})

	return tool.OK(out)
}

// isBetter mirrors FindBestByUserAndGoal sort: days ASC, money DESC, projectsCompleted DESC.
func isBetter(newDays, newMoney, newProjects, oldDays, oldMoney, oldProjects int) bool {
	if newDays != oldDays {
		return newDays < oldDays
	}
	if newMoney != oldMoney {
		return newMoney > oldMoney
	}
	return newProjects > oldProjects
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
		ID:                e.ID,
		UserID:            e.UserID,
		Nickname:          e.Nickname,
		CompanyName:       e.CompanyName,
		Days:              e.Days,
		Money:             e.Money,
		Goal:              e.Goal,
		OfficeLevel:       e.OfficeLevel,
		StaffCount:        e.StaffCount,
		ProjectsCompleted: e.ProjectsCompleted,
		SubmittedAt:       e.SubmittedAt,
	}
}

func sanityCheck(p *SubmitPayload) error {
	if p.Goal == 0 {
		p.Goal = DefaultGoal
	}
	if p.Days < 1 || p.Days > MaxDays {
		return fmt.Errorf("days must be in [1,%d] (got %d)", MaxDays, p.Days)
	}
	if _, ok := allowedGoals[p.Goal]; !ok {
		return fmt.Errorf("goal %d is not supported", p.Goal)
	}
	if p.Money < 0 {
		return fmt.Errorf("money must be >= 0 (got %d)", p.Money)
	}
	moneyMax := p.Goal*MoneyMultiplier + p.Days*MoneyPerDayCap
	if p.Money > moneyMax {
		return fmt.Errorf("money suspiciously high (money=%d, max=%d)", p.Money, moneyMax)
	}
	if p.OfficeLevel < 0 || p.OfficeLevel > MaxOfficeLevel {
		return fmt.Errorf("office_level must be in [0,%d] (got %d)", MaxOfficeLevel, p.OfficeLevel)
	}
	if p.StaffCount < 0 || p.StaffCount > MaxStaffCount {
		return fmt.Errorf("staff_count must be in [0,%d] (got %d)", MaxStaffCount, p.StaffCount)
	}
	if p.ProjectsCompleted < 0 || p.ProjectsCompleted > MaxProjects {
		return fmt.Errorf("projects_completed must be in [0,%d] (got %d)", MaxProjects, p.ProjectsCompleted)
	}
	if err := validateCompanyName(p.CompanyName); err != nil {
		return err
	}
	p.CompanyName = strings.TrimSpace(p.CompanyName)
	return nil
}

// validateCompanyName 比照前端規則：trim 後 2-8 字元（rune count）、白名單字元、無髒話。
func validateCompanyName(raw string) error {
	name := strings.TrimSpace(raw)
	if name == "" {
		return fmt.Errorf("company_name is required")
	}
	n := utf8.RuneCountInString(name)
	if n < CompanyNameMinRunes {
		return fmt.Errorf("company_name must be at least %d characters (got %d)", CompanyNameMinRunes, n)
	}
	if n > CompanyNameMaxRunes {
		return fmt.Errorf("company_name must be at most %d characters (got %d)", CompanyNameMaxRunes, n)
	}
	if !companyNameRe.MatchString(name) {
		return fmt.Errorf("company_name contains disallowed characters")
	}
	if containsProfanity(name) {
		return fmt.Errorf("company_name contains forbidden words")
	}
	return nil
}
