package main

import (
	"fmt"
	"log"
	"strings"
	"time"
	"unicode/utf8"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/xanzy/go-gitlab"
)

func sendCommitUpdate(bot *tgbotapi.BotAPI, user *User, git *gitlab.Client, project *gitlab.Project) {
	commits, _, err := git.Commits.ListCommits(project.ID,
		&gitlab.ListCommitsOptions{Since: &user.LastChecked})
	if err != nil {
		log.Printf("[Warning] Unable to load commits: %v", err.Error())
		return
	}

	for _, commit := range commits {
		if commit.CreatedAt.Before(user.LastChecked) {
			continue
		}

		description := ""
		if len([]rune(commit.Message)) > 50 {
			description = string([]rune(commit.Message)[:47]) + "..."
		} else {
			description = commit.Message
		}

		message := fmt.Sprintf("New Commit 🖥\n*%v*\n%v <%v>\n%v\n%v",
			commit.Title, commit.AuthorName, commit.AuthorEmail, description, commit.WebURL)

		sendMessage(bot, user.TelegramID, message)
	}
}

func sendIssueUpdate(bot *tgbotapi.BotAPI, user *User, git *gitlab.Client, project *gitlab.Project) {
	issues, _, err := git.Issues.ListProjectIssues(project.ID,
		&gitlab.ListProjectIssuesOptions{CreatedAfter: &user.LastChecked})
	if err != nil {
		log.Printf("[Warning] Unable to load issues: %v", err.Error())
		return
	}

	for _, issue := range issues {
		if issue.CreatedAt.Before(user.LastChecked) {
			continue
		}

		description := ""
		if len([]rune(issue.Description)) > 50 {
			description = string([]rune(issue.Description)[:47]) + "..."
		} else {
			description = issue.Description
		}

		message := fmt.Sprintf("New Issue 📨\n*%v*\n%v\n%v\n%v",
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
	case "help":
		helpCmd(bot, update)
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
		sendMessage(bot, user.TelegramID, "⚠️ Unable to verify with this token\nPlease retry")
		return
	}

	gitUser, _, err := git.Users.CurrentUser()
	if err != nil {
		sendMessage(bot, user.TelegramID, "⚠️ Unable to verify with this token\nPlease retry")
		return
	}

	// Save the new token, update the user and send a confirmation
	user.GitLabToken = token
	user.HasError = false
	user.State = UserNormal
	user.LastChecked = time.Now().Add(-30 * time.Minute)
	err = user.Save()
	if err != nil {
		sendMessage(bot, user.TelegramID, "⚠️ *Internal Error* ⚠️\nPlease retry later")
		return
	}

	message := fmt.Sprintf("*Hi %v* 👋\nThis token works. 👍\nYou can delete it any time with the /deleteGitlabToken command.", gitUser.Name)
	sendMessage(bot, user.TelegramID, message)

	// Send the user a info about the project the are subscribed to
	projectsCmd(bot, update)
}

func projectsCmd(bot *tgbotapi.BotAPI, update *tgbotapi.Update) {
	user, err := LoadUser(update.Message.Chat.ID)
	if err != nil {
		log.Printf("[Warning] Unable to load user: %s", err.Error())
		sendMessage(bot, update.Message.Chat.ID, "⚠️ *Internal Error* ⚠️\nPlease retry later")
		return
	}

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

	// Load the Projects
	options := &gitlab.ListProjectsOptions{
		Starred: gitlab.Bool(true),
	}
	projects, _, err := git.Projects.ListProjects(options)
	if err != nil {
		log.Printf("[Warning] Unable to load projects for user %v: %v", user.TelegramID, err)
		return
	}

	message := fmt.Sprintf("*Subscribed Project*\n"+
		"You are subscribed to the project you starred. "+
		"At the moment this are %v projects.\n\n", len(projects))

	// Get commit and issue updates on every project
	for i, project := range projects {
		message += fmt.Sprintf("*%v)* %v\n%v\n\n", i+1, project.NameWithNamespace, project.WebURL)
	}

	sendMessage(bot, user.TelegramID, message)
}

func startCmd(bot *tgbotapi.BotAPI, update *tgbotapi.Update) {
	setGitlabTokenCmd(bot, update)
}

func helpCmd(bot *tgbotapi.BotAPI, update *tgbotapi.Update) {
	sendMessage(bot, update.Message.Chat.ID, "😰 Not yet implemented")
}

func userInfoCmd(bot *tgbotapi.BotAPI, update *tgbotapi.Update) {
	user, err := LoadUser(update.Message.Chat.ID)
	if err != nil {
		log.Printf("[Warning] Unable to load user: %s", err.Error())
		sendMessage(bot, update.Message.Chat.ID, "⚠️ *Internal Error* ⚠️\nPlease retry later")
		return
	}

	tokenText := user.GitLabToken
	if utf8.RuneCountInString(tokenText) > 5 {
		tmpRunes := []rune(tokenText)

		tokenText = string(tmpRunes[:5]) +
			strings.Repeat("*", len(tmpRunes)-5)
		log.Print(tokenText)
	}
	if tokenText == "" {
		tokenText = "<no token>"
	}
	stateText := "_<state not found>_"
	if user.State == UserSetup {
		stateText = "Waiting for GitLab Token"
	} else if user.State == UserNormal {
		stateText = "Normal"
	}

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
	sendMessage(bot, update.Message.Chat.ID, "😰 Not yet implemented")
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
