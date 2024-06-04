// Package util ...
package util

import "strings"

// Contains ...
func Contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// Remove ...
func Remove(list []string, s string) []string {
	newList := []string{}
	for _, v := range list {
		val := strings.TrimSpace(v)
		if val != s {
			newList = append(newList, val)
		}
	}
	return newList
}

// MatchesPrefix ...
func MatchesPrefix(list []string, s string) bool {
	for _, v := range list {
		if strings.HasPrefix(s, v) {
			return true
		}
	}
	return false
}
