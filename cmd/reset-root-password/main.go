package main

import (
	"fmt"
	"log"
	"os"

	"devops-links/internal/auth"
	"devops-links/internal/config"
	"devops-links/internal/store"
)

func main() {
	configPath := os.Getenv("CONFIG_FILE")
	if configPath == "" {
		configPath = "config.yaml"
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatal(err)
	}
	st, err := store.New(cfg.Database.DSN())
	if err != nil {
		log.Fatal(err)
	}
	name := cfg.Auth.SuperAdminInitialName
	user, err := st.GetUserByName(name)
	if err != nil {
		user, err = st.GetUserByName("superadmin")
		if err != nil {
			log.Fatalf("user %q not found: %v", name, err)
		}
		if name != user.Name {
			if err := st.UpdateUserFields(user.ID, map[string]any{"name": name, "display_name": name}); err != nil {
				log.Fatalf("rename user to %q: %v", name, err)
			}
			fmt.Printf("renamed user %s -> %s\n", "superadmin", name)
			user, _ = st.GetUserByName(name)
		}
	}
	svc := auth.New(st, auth.Config{JWTSecret: cfg.Auth.JWTSecret})
	hash, err := svc.HashPassword(cfg.Auth.SuperAdminInitialPassword)
	if err != nil {
		log.Fatal(err)
	}
	if err := st.UpdateUserPassword(user.ID, hash); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("updated password for user %s\n", user.Name)
}
