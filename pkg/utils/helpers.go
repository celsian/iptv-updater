package utils

import "strings"

func ContainsSlice(title string, doNotDisables []string) bool {
	lt := strings.ToLower(title)
	for _, d := range doNotDisables {
		if strings.Contains(lt, strings.ToLower(d)) {
			return true
		}
	}

	return false
}
