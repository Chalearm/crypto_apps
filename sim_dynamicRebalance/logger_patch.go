package main

import (
	"fmt"
	"strings"
	"time"
)

// Reusable inline helper function mapping from your provided infra logger spec
func InitLogger() {
	fmt.Println("[LOGGER][INFO] Global configuration environment hooks linked for dynamic rebalance monitoring.")
}

func SetDaemonID(id string)      { fmt.Printf("[CONFIG] Daemon Context ID mapped to: %s\n", id) }
func NewCorrelationID() string   { return fmt.Sprintf("%d", time.Now().UnixNano()) }
func Info(msg string)            { fmt.Printf("[%s][INFO] %s\n", time.Now().Format("2006-01-02 15:04:05"), msg) }
func Debug(msg string)           { fmt.Printf("[%s][DEBUG] %s\n", time.Now().Format("2006-01-02 15:04:05"), msg) }
func Warn(msg string)            { fmt.Printf("[%s][WARN] %s\n", time.Now().Format("2006-01-02 15:04:05"), msg) }
func Error(msg string)           { fmt.Printf("[%s][ERROR] %s\n", time.Now().Format("2006-01-02 15:04:05"), msg) }

func formatWithSpacedDecimals(val float64) string {
	rawStr := fmt.Sprintf("%.12f", val)
	parts := strings.Split(rawStr, ".")
	intPart := parts[0]
	decPart := parts[1]

	var intResult []string
	for i, c := range intPart {
		if i > 0 && (len(intPart)-i)%3 == 0 {
			intResult = append(intResult, ",")
		}
		intResult = append(intResult, string(c))
	}
	formattedInt := strings.Join(intResult, "")

	var decResult []string
	for i, c := range decPart {
		if i > 0 && i%3 == 0 {
			decResult = append(decResult, " ")
		}
		decResult = append(decResult, string(c))
	}
	return formattedInt + "." + strings.Join(decResult, "")
}
