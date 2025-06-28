package env

import (
	"os"
	"strconv"
	"strings"
	"time"
)

func GetString(name string, defaultVal string) string {
	val := os.Getenv(name)
	if strings.TrimSpace(val) == "" {
		return defaultVal
	}

	return val
}

func GetInt(name string, defaultVal int) int {
	envVal := os.Getenv(name)

	result, err := strconv.Atoi(envVal)
	if err != nil {
		return defaultVal
	}

	return result
}

func GetBool(name string, defaultVal bool) bool { // nolint:unparam
	envVal := os.Getenv(name)

	result, err := strconv.ParseBool(envVal)
	if err != nil {
		return defaultVal
	}

	return result
}

func GetDuration(name string, defaultVal time.Duration) time.Duration { // nolint:unparam
	envVal := os.Getenv(name)

	result, err := time.ParseDuration(envVal)
	if err != nil {
		return defaultVal
	}

	return result
}
