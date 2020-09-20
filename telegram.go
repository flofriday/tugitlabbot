package main

import (
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/xanzy/go-gitlab"
)

const (
	descriptionLimit = 150
)

func sendCommitUpdate(bot *tgbotapi.BotAPI, git *gitlab.Client, user *User, gitUser *gitlab.User, project *gitlab.Project, wg *sync.WaitGroup) {
	// Update the waitgroup
	defer wg.Done()

	// Load commits from the project since the last time checked
	commits, _, err := git.Commits.ListCommits(project.ID,
		&gitlab.ListCommitsOptions{Since: &user.LastChecked})
	if err != nil {
		log.Printf("[Warning] Unable to load commits: %v", err.Error())
		return
	}

	// Send a message for every commit
	for _, commit := range commits {
		// Ignore commits already sent
		// Ignore commits from the logged in user
		if commit.CreatedAt.Before(user.LastChecked) ||
			commit.AuthorEmail == gitUser.Email {
			continue
		}

		description := cutString(commit.Message, descriptionLimit)
		message := fmt.Sprintf("New Commit 🖥\n*%v*\n%v <%v>\n%v\n%v",
			commit.Title, commit.AuthorName, commit.AuthorEmail,
			description, commit.WebURL)

		sendMessage(bot, user.TelegramID, message)
	}
}

func sendIssueUpdate(bot *tgbotapi.BotAPI, git *gitlab.Client, user *User, gitUser *gitlab.User, project *gitlab.Project, wg *sync.WaitGroup) {
	// Update the waitgroup
	defer wg.Done()

	// Load the commit since the last time we checked
	issues, _, err := git.Issues.ListProjectIssues(project.ID,
		&gitlab.ListProjectIssuesOptions{CreatedAfter: &user.LastChecked})
	if err != nil {
		log.Printf("[Warning] Unable to load issues: %v", err.Error())
		return
	}

	// Send a message for every commit
	for _, issue := range issues {
		// Ignore commits we already notified the user about
		if issue.CreatedAt.Before(user.LastChecked) {
			continue
		}

		description := cutString(issue.Description, descriptionLimit)
		message := fmt.Sprintf("New Issue ✉️\n*%v*\n%v\n%v\n%v",
			issue.Title, issue.Author.Name, description, issue.WebURL)

		sendMessage(bot, user.TelegramID, message)
	}
}

func handleUpdate(bot *tgbotapi.BotAPI, update *tgbotapi.Update) {
	if !update.Message.IsCommand() {
		handleMessage(bot, update)
		return
	}

	cmd := strings.ToLower(update.Message.Command())
	switch cmd {
	case "start":
		startCmd(bot, update)
	case "userinfo":
		userInfoCmd(bot, update)
	case "projects":
		projectsCmd(bot, update)
	case "setgitlabtoken":
		setGitlabTokenCmd(bot, update)
	case "deletegitlabtoken":
		setGitlabTokenCmd(bot, update)
	case "statistic":
		statisticCmd(bot, update)
	case "privacy":
		privacyCmd(bot, update)
	case "about":
		aboutCmd(bot, update)
	case "help":
		helpCmd(bot, update)
	default:
		message := "😅 Sorry, I didn't understand that.\nYou can type /help to see what I can understand."
		sendMessage(bot, update.Message.Chat.ID, message)
	}
}

func handleMessage(bot *tgbotapi.BotAPI, update *tgbotapi.Update) {
	user, err := LoadUser(update.Message.Chat.ID)
	if err != nil {
		log.Printf("[Warning] Unable to load user: %s", err.Error())
		return
	}

	if user.State != UserSetup {
		sendMessage(bot, user.TelegramID, "😅 Sorry, I don't know what you want.")
		return
	}

	token := strings.TrimSpace(update.Message.Text)
	git, err := gitlab.NewClient(token, gitlab.WithBaseURL("https://b3.complang.tuwien.ac.at/"))
	if err != nil {
		sendMessage(bot, user.TelegramID, tokenErrorMessage(&user))
		return
	}

	gitUser, _, err := git.Users.CurrentUser()
	if err != nil {
		sendMessage(bot, user.TelegramID, tokenErrorMessage(&user))
		return
	}

	// Save the new token, update the user and send a confirmation
	user.GitLabToken = token
	user.HasError = false
	user.State = UserNormal
	user.LastChecked = time.Now()
	err = user.Save()
	if err != nil {
		sendMessage(bot, user.TelegramID, "⚠️ *Internal Error* ⚠️\n"+
			"Please retry later")
		return
	}

	message := fmt.Sprintf("*Hi %v* 👋\nThis token works. "+
		"👍\nYou can delete it any time with the /deleteGitlabToken command. "+
		"I will notify you as soon as something happens on your repos.",
		gitUser.Name)
	sendMessage(bot, user.TelegramID, message)

	// Send the user a info about the project the are subscribed to
	projectsCmd(bot, update)
}

func projectsCmd(bot *tgbotapi.BotAPI, update *tgbotapi.Update) {
	// Load the user from disk
	user, err := LoadUser(update.Message.Chat.ID)
	if err != nil {
		log.Printf("[Warning] Unable to load user: %s", err.Error())
		sendMessage(bot, update.Message.Chat.ID, "⚠️ *Internal Error* ⚠️\nPlease retry later")
		return
	}

	// Check if the user even has a token.
	if user.GitLabToken == "" {
		message := "Sorry, I need a GitLabToken for this command to work. 😅\n" +
			"You can set a token with /setGitLabToken."
		sendMessage(bot, user.TelegramID, message)
		return
	}

	// Create a GitLab client
	git, err := gitlab.NewClient(user.GitLabToken, gitlab.WithBaseURL("https://b3.complang.tuwien.ac.at/"))
	if err != nil {
		log.Printf("[Error] Unable to authenticate for user %v: %v", user.TelegramID, err.Error())
		sendMessage(bot, user.TelegramID, tokenErrorMessage(&user))
		return
	}

	// Try to get the current user to ensure we are logged in, as this
	// is only available for voalid tokens
	_, _, err = git.Users.CurrentUser()
	if err != nil {
		sendMessage(bot, user.TelegramID, tokenErrorMessage(&user))
		user.HasError = true
		user.Save()
		return
	}

	// Load the Projects
	options := &gitlab.ListProjectsOptions{
		Starred: gitlab.Bool(true),
	}
	projects, _, err := git.Projects.ListProjects(options)
	if err != nil {
		sendMessage(bot, update.Message.Chat.ID, "⚠️ *Internal Error* ⚠️\nPlease retry later")
		log.Printf("[Warning] Unable to load projects for user %v: %v", user.TelegramID, err)
		return
	}

	// Create the message
	message := fmt.Sprintf("*Subscribed Project*\n"+
		"You are subscribed to the project you have starred. "+
		"At the moment this are %v projects.\n\n", len(projects))
	for i, project := range projects {
		message += fmt.Sprintf("*%v)* %v\n%v\n\n", i+1, project.NameWithNamespace, project.WebURL)
	}

	sendMessage(bot, user.TelegramID, message)
}

func startCmd(bot *tgbotapi.BotAPI, update *tgbotapi.Update) {
	aboutCmd(bot, update)

	user, err := LoadUser(update.Message.Chat.ID)
	if err != nil {
		log.Printf("[Warning] Unable to load user: %s", err.Error())
		sendMessage(bot, update.Message.Chat.ID, "⚠️ *Internal Error* ⚠️\nPlease retry later")
		return
	}

	if user.GitLabToken == "" {
		setGitlabTokenCmd(bot, update)
	}
}

func aboutCmd(bot *tgbotapi.BotAPI, update *tgbotapi.Update) {
	message := "🎉 *TUGitLabBot* 🎉\n" +
		"This bot sends you messages if new issues or commits get created on your " +
		"TU GitLab repositories.\nThis bot was created by " +
		"[@flofriday](https://github.com/flofriday) in the hope to be useful " +
		"and its code is publicly available at: " +
		"https://github.com/flofriday/tugitlabbot" +
		"\n\n" +
		"You can find a list with all commands with the /help command." +
		"\n\n" +
		"*Disclaimer:* This is not an official offer from TU Wien!"
	sendMessage(bot, update.Message.Chat.ID, message)
}

func helpCmd(bot *tgbotapi.BotAPI, update *tgbotapi.Update) {
	message := "*Help*\n" +
		"/projects - List the project you are subscribed to\n" +
		"/setgitlabtoken - Set your GitLab token\n" +
		"/deletegitlabtoken - Delete your GitLab token\n" +
		"/userinfo - Show all the info this bot has about you\n" +
		"/statistic - Show statistics about the bot\n" +
		"/privacy - How this bot handles privacy\n" +
		"/about - About this bot\n" +
		"/help - This help message\n"

	sendMessage(bot, update.Message.Chat.ID, message)
}

func userInfoCmd(bot *tgbotapi.BotAPI, update *tgbotapi.Update) {
	// Load the user from disk
	user, err := LoadUser(update.Message.Chat.ID)
	if err != nil {
		log.Printf("[Warning] Unable to load user: %s", err.Error())
		sendMessage(bot, update.Message.Chat.ID, "⚠️ *Internal Error* ⚠️\nPlease retry later")
		return
	}

	// Censor the GitLabToken and convert the userstate into something readable
	tokenText := censorString(user.GitLabToken)
	if tokenText == "" {
		tokenText = "<no token>"
	}
	stateText := "_<state not found>_"
	if user.State == UserSetup {
		stateText = "Waiting for GitLab Token"
	} else if user.State == UserNormal {
		stateText = "Normal"
	}

	// Send the message
	message := fmt.Sprintf("*User Info*\n"+"TelegramID: `%v`\n"+
		"GitLab Token: `%v`\n"+"Has Error: %v\n"+"State: %v\n"+"Last updated: %v\n",
		user.TelegramID, tokenText, user.HasError, stateText, user.LastChecked.Format(time.RFC1123))
	sendMessage(bot, user.TelegramID, message)
}

func setGitlabTokenCmd(bot *tgbotapi.BotAPI, update *tgbotapi.Update) {
	user, err := LoadUser(update.Message.Chat.ID)
	if err != nil {
		log.Printf("[Warning] Unable to load user: %s", err.Error())
		return
	}

	if user.State != UserSetup {
		user.State = UserSetup
		err = user.Save()
		if err != nil {
			sendMessage(bot, user.TelegramID, "⚠️ *Internal Error* ⚠️\nPlease retry later")
			return
		}
	}

	message := "*Gitlab Token*\n1) Go into your GitLab Profile Settings\n" +
		"2) Select 'Access Tokens' in the Sidebar left\n" +
		"3) Create a new token with the scope 'api'\n" +
		"4) Send me this token\n"
	sendMessage(bot, user.TelegramID, message)
}

func deleteGitlabTokenCmd(bot *tgbotapi.BotAPI, update *tgbotapi.Update) {
	user, err := LoadUser(update.Message.Chat.ID)
	if err != nil {
		log.Printf("[Warning] Unable to load user: %s", err.Error())
		sendMessage(bot, update.Message.Chat.ID, "⚠️ *Internal Error* ⚠️\nPlease retry later")
		return
	}

	user.GitLabToken = ""
	err = user.Save()
	if err != nil {
		sendMessage(bot, user.TelegramID, "⚠️ *Internal Error* ⚠️\nPlease retry later")
		return
	}

	sendMessage(bot, user.TelegramID, "GitLab Token deleted 👍")
}

func statisticCmd(bot *tgbotapi.BotAPI, update *tgbotapi.Update) {
	// Load all users from disk
	users, err := LoadAllUsers()
	if err != nil {
		log.Printf("[Warning] Unable to load all user: %s", err.Error())
		sendMessage(bot, update.Message.Chat.ID, "⚠️ *Internal Error* ⚠️\nPlease retry later")
		return
	}

	// Load some statistics
	numErrors := 0
	numTokens := 0
	numUsers := len(users)
	for _, user := range users {
		if user.HasError {
			numErrors++
		}
		if user.GitLabToken != "" {
			numTokens++
		}
	}

	// Create a message
	message := fmt.Sprintf("*Statistic*\nUser: %v\n"+
		"User with Token:: %v\n"+
		"User with Error: %v",
		numUsers, numTokens, numErrors)
	sendMessage(bot, update.Message.Chat.ID, message)
}

func privacyCmd(bot *tgbotapi.BotAPI, update *tgbotapi.Update) {
	sendMessage(bot, update.Message.Chat.ID, "😰 Not yet implemented")
}

func sendMessage(bot *tgbotapi.BotAPI, telegramID int64, text string) {
	msg := tgbotapi.NewMessage(telegramID, text)
	msg.ParseMode = "Markdown"
	msg.DisableWebPagePreview = true
	_, err := bot.Send(msg)
	if err != nil {
		log.Printf("[Warning] Couldn't send a message: %v", err.Error())
	}
}
