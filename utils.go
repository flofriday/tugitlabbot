package main

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// Censor a String by cover every char by a star after the fifth char
// Example: abcdefghij -> abcde*****
func censorString(s string) string {
	if utf8.RuneCountInString(s) > 5 {
		tmp := []rune(s)
		s = string(tmp[:5]) + strings.Repeat("*", len(tmp)-5)
	}
	return s
}

// Cut a string if it exceeds the limit, and replace the last three elements
// by dots
func cutString(s string, limit int) string {
	if len([]rune(s)) > limit {
		return string([]rune(s)[:limit-3]) + "..."
	}
	return s
}

func tokenErrorMessage(u *User) string {
	return "⚠️ Unable to log in with your saved GitLab token!\n" +
		"Maybe your token expired recently?\n" +
		fmt.Sprintf("Token: `%v`", censorString(u.GitLabToken))
}
