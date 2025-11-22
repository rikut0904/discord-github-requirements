package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github-discord-bot/internal/domain/repository"
	"github-discord-bot/internal/infrastructure/crypto"
	"github-discord-bot/internal/infrastructure/database"
	"github-discord-bot/internal/interface/handler"
	"github-discord-bot/internal/usecase"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	// Load environment variables
	discordToken := os.Getenv("DISCORD_TOKEN")
	if discordToken == "" {
		log.Fatal("DISCORD_TOKEN is required")
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	encryptionKey := os.Getenv("ENCRYPTION_KEY")
	if encryptionKey == "" {
		log.Fatal("ENCRYPTION_KEY is required")
	}

	// Initialize database
	db, err := database.InitDB(databaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Initialize crypto
	aesCrypto, err := crypto.NewAESCrypto(encryptionKey)
	if err != nil {
		log.Fatalf("Failed to initialize encryption: %v", err)
	}

	// Initialize repository
	var userSettingRepo repository.UserSettingRepository = database.NewPostgresUserSettingRepository(db)

	// Initialize usecases
	settingUsecase := usecase.NewSettingUsecase(userSettingRepo, aesCrypto)
	issuesUsecase := usecase.NewIssuesUsecase(userSettingRepo, aesCrypto)

	// Initialize Discord session
	dg, err := discordgo.New("Bot " + discordToken)
	if err != nil {
		log.Fatalf("Failed to create Discord session: %v", err)
	}

	// Initialize handler
	discordHandler := handler.NewDiscordHandler(settingUsecase, issuesUsecase)

	// Register handlers
	dg.AddHandler(discordHandler.HandleInteraction)
	dg.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		fmt.Printf("Bot is ready! Logged in as %s\n", s.State.User.Username)
	})

	// Open connection
	err = dg.Open()
	if err != nil {
		log.Fatalf("Failed to open Discord connection: %v", err)
	}
	defer dg.Close()

	// Register commands
	err = discordHandler.RegisterCommands(dg)
	if err != nil {
		log.Fatalf("Failed to register commands: %v", err)
	}

	fmt.Println("Bot is now running. Press CTRL-C to exit.")

	// Wait for interrupt signal
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc
}
