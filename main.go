package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/go-co-op/gocron"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/xanzy/go-gitlab"
)

func runTask(bot *tgbotapi.BotAPI, user *User) {
	// Authenticate to the GitLab API
	git, err := gitlab.NewClient(user.GitLabToken, gitlab.WithBaseURL("https://b3.complang.tuwien.ac.at/"))
	if err != nil {
		log.Printf("[Error] Unable to authenticate for user %v: %v", user.TelegramID, err.Error())

		// Send a error message to the user if not already sent
		if user.HasError {
			return
		}
		sendMessage(bot, user.TelegramID, fmt.Sprintf("Unable to authenticate with your token.\nRaw error: %v", err.Error()))
		user.HasError = true
		user.Save()
		return
	}

	// Update the user
	if user.HasError {
		user.HasError = false
		user.Save()
	}

	// Load the Projects
	options := &gitlab.ListProjectsOptions{
		Starred: gitlab.Bool(true),
	}
	projects, _, err := git.Projects.ListProjects(options)
	if err != nil {
		log.Printf("[Warning] Unable to load projects for user %v: %v", user.TelegramID, err)
		return
	}

	// Get commit and issue updates on every project
	for _, project := range projects {
		sendCommitUpdate(bot, user, git, project)
		sendIssueUpdate(bot, user, git, project)
	}

	user.LastChecked = time.Now()
	user.Save()
}

func runTasks(bot *tgbotapi.BotAPI) {
	log.Print("[Info] Run background job")

	// Load all users
	users, err := LoadAllUsers()
	if err != nil {
		log.Println("[Error] Unable to load users from disk.")
	}

	// Run the task for every user
	for _, user := range users {
		// Ignore users that don't have a GitLab Token
		if user.GitLabToken == "" {
			continue
		}

		go runTask(bot, &user)
	}
}

func main() {
	// Setup the Telegrambot
	bot, err := tgbotapi.NewBotAPI(os.Getenv("TELEGRAM_TOKEN"))
	if err != nil {
		log.Fatalf("Unable to authorize as telegram bot: %v", err.Error())
		return
	}
	log.Printf("Authorized on account %s", bot.Self.UserName)
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	// Setup the user DB
	err = InitUsers()
	if err != nil {
		log.Fatalf("Unable to initialize user db: %v", err.Error())
		return
	}

	// Start the background jobs
	s1 := gocron.NewScheduler(time.UTC)
	s1.Every(15).Minutes().Do(runTasks, bot)
	s1.StartAsync()
	runTasks(bot)

	// Listen for Telegramm events
	updates, err := bot.GetUpdatesChan(u)
	for update := range updates {
		if update.Message == nil {
			continue
		}

		log.Printf("[%s] %s", update.Message.From.UserName, update.Message.Text)

		go handleUpdate(bot, &update)
	}
}
