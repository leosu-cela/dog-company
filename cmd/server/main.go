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

	"github.com/gin-gonic/gin"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"

	"github.com/leosu-cela/dog-company/internal/auth"
	"github.com/leosu-cela/dog-company/internal/config"
	"github.com/leosu-cela/dog-company/internal/database"
	"github.com/leosu-cela/dog-company/internal/user"
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

	jwtImpl := auth.NewHS256JWT(cfg.JWTSecret, cfg.JWTTTL)
	userRepo := user.NewUserRepository()
	userHandler := user.NewUserHandler(db, userRepo, jwtImpl)
	userCtrl := user.NewUserController(userHandler)

	r := gin.Default()

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

	api := r.Group("/api/v1")

	authPublic := api.Group("/auth")
	authPublic.POST("/register", userCtrl.Register)
	authPublic.POST("/login", userCtrl.Login)

	authed := api.Group("", auth.AuthRequired(jwtImpl))
	authed.GET("/auth/me", userCtrl.Me)

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
