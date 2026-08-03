package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"devops-links/internal/auth"
	"devops-links/internal/config"
	"devops-links/internal/httpapi"
	"devops-links/internal/ldapauth"
	"devops-links/internal/store"

	"github.com/gin-gonic/gin"
)

func main() {
	configPath := os.Getenv("CONFIG_FILE")
	if configPath == "" {
		configPath = "config.yaml"
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	if err := store.EnsureDatabase(cfg.Database.Host, cfg.Database.Port, cfg.Database.User, cfg.Database.Password, cfg.Database.Name, cfg.Database.Charset); err != nil {
		log.Fatalf("ensure database: %v", err)
	}
	st, err := store.New(cfg.Database.DSN())
	if err != nil {
		log.Fatalf("open database: %v", err)
	}

	expiry, err := time.ParseDuration(cfg.Auth.TokenExpiry)
	if err != nil {
		log.Fatalf("parse token_expiry: %v", err)
	}
	authSvc := auth.New(st, auth.Config{
		JWTSecret:   cfg.Auth.JWTSecret,
		TokenExpiry: expiry,
		SMTPHost:    cfg.Auth.SMTPHost,
		SMTPPort:    cfg.Auth.SMTPPort,
		SMTPUser:    cfg.Auth.SMTPUser,
		SMTPPass:    cfg.Auth.SMTPPass,
		SMTPFrom:    cfg.Auth.SMTPFrom,
	})
	ldapClient := ldapauth.New(cfg.Auth.LDAP)
	if ldapClient.Enabled() {
		log.Printf("LDAP auth enabled: host=%s base_dn=%s label=%s", cfg.Auth.LDAP.Host, cfg.Auth.LDAP.BaseDN, ldapClient.Label())
	} else {
		log.Printf("LDAP auth disabled")
	}
	handler := httpapi.NewHandler(cfg, st, authSvc, ldapClient)
	if err := handler.EnsureInitialUsers(); err != nil {
		log.Fatalf("ensure users: %v", err)
	}

	if err := os.MkdirAll(cfg.Upload.AbsDir(), 0o755); err != nil {
		log.Fatalf("create upload dir: %v", err)
	}

	router := gin.Default()
	handler.RegisterRoutes(router)
	registerFrontend(router)

	addr := fmt.Sprintf(":%d", cfg.Server.Port)
	log.Printf("devops-links server started at %s", addr)
	if err := router.Run(addr); err != nil {
		log.Fatalf("start server: %v", err)
	}
}

func registerFrontend(router *gin.Engine) {
	distPath := "frontend/dist"
	if _, err := os.Stat(distPath); err != nil {
		return
	}
	router.Static("/assets", distPath+"/assets")
	router.GET("/", func(c *gin.Context) {
		c.File(distPath + "/index.html")
	})
	router.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		if len(path) >= 4 && path[:4] == "/api" {
			c.JSON(404, gin.H{"error": "not found"})
			return
		}
		if len(path) >= 8 && path[:8] == "/uploads" {
			c.JSON(404, gin.H{"error": "not found"})
			return
		}
		c.File(distPath + "/index.html")
	})
}
