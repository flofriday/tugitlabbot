package main

import (
	"log"
	"os"
	"sync"
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
		sendMessage(bot, user.TelegramID, tokenErrorMessage(user))
		user.HasError = true
		user.Save()
		return
	}

	// Update the user
	if user.HasError {
		user.HasError = false
		user.Save()
	}

	// Load the logged in user informations
	gitUser, _, err := git.Users.CurrentUser()
	if err != nil {
		sendMessage(bot, user.TelegramID, tokenErrorMessage(user))
		return
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
	tmpTime := time.Now()
	var wg sync.WaitGroup
	for _, project := range projects {
		wg.Add(2)
		go func(bot *tgbotapi.BotAPI, git *gitlab.Client, user *User, gitUser *gitlab.User, project *gitlab.Project) {
			defer wg.Done()
			sendCommitUpdate(bot, git, user, gitUser, project)
		}(bot, git, user, gitUser, project)
		go func(bot *tgbotapi.BotAPI, git *gitlab.Client, user *User, gitUser *gitlab.User, project *gitlab.Project) {
			defer wg.Done()
			sendIssueUpdate(bot, git, user, gitUser, project)
		}(bot, git, user, gitUser, project)
	}
	wg.Wait()

	user.LastChecked = tmpTime
	user.Save()
}

func runTasks(bot *tgbotapi.BotAPI) {
	// Load all users
	users, err := LoadAllUsers()
	if err != nil {
		log.Println("[Error] Unable to load users from disk for background job.")
	}
	log.Print("[Info] Run background job...")

	// Run the task for every user
	var wg sync.WaitGroup
	counter := 0
	for _, user := range users {
		// Ignore users that don't have a GitLab Token
		if user.GitLabToken == "" {
			continue
		}

		counter++
		wg.Add(1)
		go func(bot *tgbotapi.BotAPI, user User) {
			defer wg.Done()
			runTask(bot, &user)
		}(bot, user)
	}
	wg.Wait()
	log.Printf("[Info] Completed background job for %v users", counter)
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
