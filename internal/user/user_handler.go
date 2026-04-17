package user

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/leosu-cela/dog-company/internal/auth"
	"github.com/leosu-cela/dog-company/pkg/tool"
)

const bcryptCost = 10

type RegisterInput struct {
	Account  string
	Password string
}

type RegisterOutput struct {
	UserID  uint64 `json:"user_id"`
	UID     string `json:"uid"`
	Account string `json:"account"`
}

type LoginInput struct {
	Account  string
	Password string
}

type LoginOutput struct {
	AccessToken      string    `json:"access_token"`
	AccessExpiresAt  time.Time `json:"access_expires_at"`
	RefreshToken     string    `json:"refresh_token"`
	RefreshExpiresAt time.Time `json:"refresh_expires_at"`
}

type RefreshInput struct {
	RefreshToken string
}

type RefreshOutput struct {
	AccessToken      string    `json:"access_token"`
	AccessExpiresAt  time.Time `json:"access_expires_at"`
	RefreshToken     string    `json:"refresh_token"`
	RefreshExpiresAt time.Time `json:"refresh_expires_at"`
}

type LogoutInput struct {
	RefreshToken string
}

type MeOutput struct {
	UserID  uint64 `json:"user_id"`
	UID     string `json:"uid"`
	Account string `json:"account"`
}

type UserHandler struct {
	db          *gorm.DB
	repo        IUserRepository
	refreshRepo auth.IRefreshTokenRepository
	signer      auth.Signer
	refreshTTL  time.Duration
}

func NewUserHandler(db *gorm.DB, repo IUserRepository, refreshRepo auth.IRefreshTokenRepository, signer auth.Signer, refreshTTL time.Duration) *UserHandler {
	return &UserHandler{
		db:          db,
		repo:        repo,
		refreshRepo: refreshRepo,
		signer:      signer,
		refreshTTL:  refreshTTL,
	}
}

func (handler *UserHandler) Register(ctx context.Context, in RegisterInput) (RegisterOutput, tool.CommonResponse) {
	group := "[UserHandler@Register]"

	account := tool.NormalizeAccount(in.Account)
	if err := tool.ValidateAccount(account); err != nil {
		return RegisterOutput{}, tool.Err(tool.CodeBadRequest, err.Error())
	}
	if err := tool.ValidatePassword(in.Password); err != nil {
		return RegisterOutput{}, tool.Err(tool.CodeBadRequest, err.Error())
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcryptCost)
	if err != nil {
		log.Printf("%s bcrypt failed: %v", group, err)
		return RegisterOutput{}, tool.Err(tool.CodeInternal, "internal error")
	}

	u := &User{
		UID:          uuid.New(),
		Account:      account,
		PasswordHash: string(hash),
	}

	tx := handler.db.WithContext(ctx)
	if err := handler.repo.Create(tx, u); err != nil {
		if errors.Is(err, ErrDuplicated) {
			return RegisterOutput{}, tool.Err(tool.CodeConflict, "account already exists")
		}
		log.Printf("%s repo.Create failed: %v", group, err)
		return RegisterOutput{}, tool.Err(tool.CodeInternal, "internal error")
	}

	return RegisterOutput{UserID: u.ID, UID: u.UID.String(), Account: u.Account}, tool.OK(nil)
}

func (handler *UserHandler) Login(ctx context.Context, in LoginInput) (LoginOutput, tool.CommonResponse) {
	group := "[UserHandler@Login]"

	account := tool.NormalizeAccount(in.Account)
	if err := tool.ValidateAccount(account); err != nil {
		return LoginOutput{}, tool.Err(tool.CodeUnauthorized, "account or password incorrect")
	}
	if err := tool.ValidatePassword(in.Password); err != nil {
		return LoginOutput{}, tool.Err(tool.CodeUnauthorized, "account or password incorrect")
	}

	tx := handler.db.WithContext(ctx)
	u, err := handler.repo.FindByAccount(tx, account)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return LoginOutput{}, tool.Err(tool.CodeUnauthorized, "account or password incorrect")
		}
		log.Printf("%s repo.FindByAccount failed: %v", group, err)
		return LoginOutput{}, tool.Err(tool.CodeInternal, "internal error")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(in.Password)); err != nil {
		return LoginOutput{}, tool.Err(tool.CodeUnauthorized, "account or password incorrect")
	}

	accessToken, accessExpiresAt, err := handler.signer.Sign(u.UID)
	if err != nil {
		log.Printf("%s signer.Sign failed: %v", group, err)
		return LoginOutput{}, tool.Err(tool.CodeInternal, "internal error")
	}

	refreshRaw, refreshHash, err := auth.GenerateRefreshToken()
	if err != nil {
		log.Printf("%s GenerateRefreshToken failed: %v", group, err)
		return LoginOutput{}, tool.Err(tool.CodeInternal, "internal error")
	}
	refreshExpiresAt := time.Now().Add(handler.refreshTTL)
	if err := handler.refreshRepo.Create(tx, &auth.RefreshToken{
		UserID:    u.ID,
		TokenHash: refreshHash,
		ExpiresAt: refreshExpiresAt,
	}); err != nil {
		log.Printf("%s refreshRepo.Create failed: %v", group, err)
		return LoginOutput{}, tool.Err(tool.CodeInternal, "internal error")
	}

	return LoginOutput{
		AccessToken:      accessToken,
		AccessExpiresAt:  accessExpiresAt,
		RefreshToken:     refreshRaw,
		RefreshExpiresAt: refreshExpiresAt,
	}, tool.OK(nil)
}

// Refresh rotates the refresh token: marks the old one revoked + linked to
// a fresh one, issues a new access token. If the presented refresh token is
// already revoked, treats it as theft and revokes all refresh tokens for
// that user (OAuth 2.1 reuse detection).
func (handler *UserHandler) Refresh(ctx context.Context, in RefreshInput) (RefreshOutput, tool.CommonResponse) {
	group := "[UserHandler@Refresh]"

	if in.RefreshToken == "" {
		return RefreshOutput{}, tool.Err(tool.CodeBadRequest, "refresh_token required")
	}
	hash := auth.HashRefreshToken(in.RefreshToken)
	tx := handler.db.WithContext(ctx)

	existing, err := handler.refreshRepo.FindByHash(tx, hash)
	if err != nil {
		if errors.Is(err, auth.ErrRefreshNotFound) {
			return RefreshOutput{}, tool.Err(tool.CodeUnauthorized, "invalid refresh token")
		}
		log.Printf("%s FindByHash failed: %v", group, err)
		return RefreshOutput{}, tool.Err(tool.CodeInternal, "internal error")
	}

	if existing.RevokedAt != nil {
		log.Printf("%s reuse detected user=%d rt_id=%d", group, existing.UserID, existing.ID)
		if err := handler.refreshRepo.RevokeAllByUser(tx, existing.UserID); err != nil {
			log.Printf("%s RevokeAllByUser failed: %v", group, err)
		}
		return RefreshOutput{}, tool.Err(tool.CodeUnauthorized, "refresh token reuse detected; please login again")
	}

	if time.Now().After(existing.ExpiresAt) {
		return RefreshOutput{}, tool.Err(tool.CodeUnauthorized, "refresh token expired")
	}

	newRaw, newHash, err := auth.GenerateRefreshToken()
	if err != nil {
		log.Printf("%s GenerateRefreshToken failed: %v", group, err)
		return RefreshOutput{}, tool.Err(tool.CodeInternal, "internal error")
	}
	newExpiresAt := time.Now().Add(handler.refreshTTL)

	var accessToken string
	var accessExpiresAt time.Time

	txErr := tx.Transaction(func(itx *gorm.DB) error {
		u, err := handler.repo.FindByID(itx, existing.UserID)
		if err != nil {
			return err
		}
		if err := handler.refreshRepo.Create(itx, &auth.RefreshToken{
			UserID:    existing.UserID,
			TokenHash: newHash,
			ExpiresAt: newExpiresAt,
		}); err != nil {
			return err
		}
		if err := handler.refreshRepo.Revoke(itx, existing.ID, &newHash); err != nil {
			return err
		}
		var signErr error
		accessToken, accessExpiresAt, signErr = handler.signer.Sign(u.UID)
		return signErr
	})

	if txErr != nil {
		log.Printf("%s rotate tx failed: %v", group, txErr)
		return RefreshOutput{}, tool.Err(tool.CodeInternal, "internal error")
	}

	return RefreshOutput{
		AccessToken:      accessToken,
		AccessExpiresAt:  accessExpiresAt,
		RefreshToken:     newRaw,
		RefreshExpiresAt: newExpiresAt,
	}, tool.OK(nil)
}

// Logout is idempotent: unknown / already-revoked tokens return OK silently
// so the client can safely call it without extra state checks.
func (handler *UserHandler) Logout(ctx context.Context, in LogoutInput) tool.CommonResponse {
	group := "[UserHandler@Logout]"

	if in.RefreshToken == "" {
		return tool.OK(nil)
	}

	hash := auth.HashRefreshToken(in.RefreshToken)
	tx := handler.db.WithContext(ctx)

	existing, err := handler.refreshRepo.FindByHash(tx, hash)
	if err != nil {
		if errors.Is(err, auth.ErrRefreshNotFound) {
			return tool.OK(nil)
		}
		log.Printf("%s FindByHash failed: %v", group, err)
		return tool.Err(tool.CodeInternal, "internal error")
	}

	if existing.RevokedAt != nil {
		return tool.OK(nil)
	}

	if err := handler.refreshRepo.Revoke(tx, existing.ID, nil); err != nil {
		log.Printf("%s Revoke failed: %v", group, err)
		return tool.Err(tool.CodeInternal, "internal error")
	}
	return tool.OK(nil)
}

func (handler *UserHandler) Me(ctx context.Context, uid uuid.UUID) (MeOutput, tool.CommonResponse) {
	group := "[UserHandler@Me]"

	tx := handler.db.WithContext(ctx)
	u, err := handler.repo.FindByUID(tx, uid)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return MeOutput{}, tool.Err(tool.CodeUnauthorized, "user not found")
		}
		log.Printf("%s repo.FindByUID failed: %v", group, err)
		return MeOutput{}, tool.Err(tool.CodeInternal, "internal error")
	}

	return MeOutput{UserID: u.ID, UID: u.UID.String(), Account: u.Account}, tool.OK(nil)
}
