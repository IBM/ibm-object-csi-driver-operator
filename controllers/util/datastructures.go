// Package util ...
package util

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
	var newList []string
	for _, v := range list {
		if v != s {
			newList = append(newList, v)
		}
	}
	return newList
}
