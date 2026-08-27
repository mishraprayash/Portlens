package render

import (
	"os"
	"time"
)

func reportNow() time.Time { return time.Now() }

var cachedHome = os.Getenv("HOME")

func userHomeDir() string {
	if cachedHome != "" {
		return cachedHome
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	cachedHome = home
	return home
}
