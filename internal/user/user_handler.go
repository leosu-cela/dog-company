package user

import (
	"context"
	"errors"
	"log"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/leosu-cela/dog-company/internal/auth"
	"github.com/leosu-cela/dog-company/pkg/tool"
)

const bcryptCost = 12

type RegisterInput struct {
	Account  string
	Password string
}

type RegisterOutput struct {
	UserID  uint64 `json:"user_id"`
	Account string `json:"account"`
}

type LoginInput struct {
	Account  string
	Password string
}

type LoginOutput struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

type MeOutput struct {
	UserID  uint64 `json:"user_id"`
	Account string `json:"account"`
}

type UserHandler struct {
	db     *gorm.DB
	repo   IUserRepository
	signer auth.Signer
}

func NewUserHandler(db *gorm.DB, repo IUserRepository, signer auth.Signer) *UserHandler {
	return &UserHandler{db: db, repo: repo, signer: signer}
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

	return RegisterOutput{UserID: u.ID, Account: u.Account}, tool.OK(nil)
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

	token, exp, err := handler.signer.Sign(u.ID)
	if err != nil {
		log.Printf("%s signer.Sign failed: %v", group, err)
		return LoginOutput{}, tool.Err(tool.CodeInternal, "internal error")
	}

	return LoginOutput{Token: token, ExpiresAt: exp}, tool.OK(nil)
}

func (handler *UserHandler) Me(ctx context.Context, userID uint64) (MeOutput, tool.CommonResponse) {
	group := "[UserHandler@Me]"

	tx := handler.db.WithContext(ctx)
	u, err := handler.repo.FindByID(tx, userID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return MeOutput{}, tool.Err(tool.CodeUnauthorized, "user not found")
		}
		log.Printf("%s repo.FindByID failed: %v", group, err)
		return MeOutput{}, tool.Err(tool.CodeInternal, "internal error")
	}

	return MeOutput{UserID: u.ID, Account: u.Account}, tool.OK(nil)
}
