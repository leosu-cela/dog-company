// Package main is the entry point for the dog-company API server.
//
//	@title			dog-company API
//	@version		1.0
//	@description	Member system for dog-company (register, login, profile).
//	@BasePath		/api/v1
//
//	@securityDefinitions.apikey	BearerAuth
//	@in							header
//	@name						Authorization
//	@description				Type "Bearer {token}" (without quotes).
//
//go:generate swag init -g cmd/server/main.go -o docs --parseDependency --parseInternal
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "github.com/leosu-cela/dog-company/docs"
	"github.com/leosu-cela/dog-company/internal/auth"
	"github.com/leosu-cela/dog-company/internal/config"
	"github.com/leosu-cela/dog-company/internal/database"
	"github.com/leosu-cela/dog-company/internal/leaderboard"
	"github.com/leosu-cela/dog-company/internal/save"
	"github.com/leosu-cela/dog-company/internal/user"
	"github.com/leosu-cela/dog-company/pkg/tool"
)

func main() {
	cfg := config.Load()

	if err := runMigrations(cfg.DatabaseURL); err != nil {
		log.Fatalf("migrations failed: %v", err)
	}

	db, err := database.New(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database init failed: %v", err)
	}

	jwtImpl := auth.NewHS256JWT(cfg.JWTSecret, cfg.JWTAccessTTL)
	userRepo := user.NewUserRepository()
	refreshRepo := auth.NewRefreshTokenRepository()
	userHandler := user.NewUserHandler(db, userRepo, refreshRepo, jwtImpl, cfg.JWTRefreshTTL)
	userCtrl := user.NewUserController(userHandler)

	saveRepo := save.NewSaveRepository()
	saveHandler := save.NewSaveHandler(db, saveRepo)
	saveCtrl := save.NewSaveController(saveHandler)

	leaderboardRepo := leaderboard.NewEntryRepository()
	leaderboardCache := leaderboard.NewListCache(leaderboard.ListCacheTTL)
	leaderboardHandler := leaderboard.NewLeaderboardHandler(db, leaderboardRepo, userRepo, leaderboardCache)
	leaderboardCtrl := leaderboard.NewLeaderboardController(leaderboardHandler)

	r := gin.Default()
	r.Use(cors.New(buildCORSConfig(cfg.CORSOrigins)))

	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "hello from dog-company"})
	})

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	r.GET("/ready", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if err := database.Ping(ctx, db); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "db ping failed", "error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ready"})
	})

	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	api := r.Group("/api/v1")

	authPublic := api.Group("/auth")
	authPublic.POST("/register", userCtrl.Register)
	authPublic.POST("/login", userCtrl.Login)
	authPublic.POST("/refresh", userCtrl.Refresh)
	authPublic.POST("/logout", userCtrl.Logout)

	api.GET("/leaderboard", auth.AuthOptional(jwtImpl), leaderboardCtrl.List)

	authed := api.Group("", auth.AuthRequired(jwtImpl))
	authed.GET("/auth/me", userCtrl.Me)

	authed.GET("/saves", saveCtrl.Get)
	authed.POST("/saves", tool.MaxBodySize(128*1024), saveCtrl.Upsert)
	authed.DELETE("/saves", saveCtrl.Delete)

	authed.POST("/leaderboard", leaderboardCtrl.Submit)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("server listening on :%s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("forced shutdown: %v", err)
	}
	log.Println("server exited")
}

func buildCORSConfig(origins []string) cors.Config {
	c := cors.Config{
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "Accept"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
	}
	if len(origins) == 0 {
		c.AllowAllOrigins = true
	} else {
		c.AllowOrigins = origins
	}
	return c
}

func runMigrations(databaseURL string) error {
	m, err := migrate.New("file://migrations", databaseURL)
	if err != nil {
		return err
	}
	defer m.Close()
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}
	log.Println("migrations applied successfully")
	return nil
}
