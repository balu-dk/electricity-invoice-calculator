// lib/utils/console.go
package utils

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// ANSI Color codes
var (
	Reset   = "\033[0m"
	Red     = "\033[31m"
	Green   = "\033[32m"
	Yellow  = "\033[33m"
	Blue    = "\033[34m"
	Magenta = "\033[35m"
	Cyan    = "\033[36m"
	Gray    = "\033[37m"
	White   = "\033[97m"
)

// Clears the terminal screen
func ClearConsole() {
	fmt.Print("\033[H\033[2J")
}

// Prints message in white
func PrintAction(message string) {
	fmt.Println(White + message + Reset)
}

// Prints message in green
func PrintSuccess(message string) {
	fmt.Println(Green + message + Reset)
}

// Prints message in blue
func PrintInfo(message string) {
	fmt.Println(Blue + message + Reset)
}

// Prints message in yellow
func PrintWarning(message string) {
	fmt.Println(Yellow + message + Reset)
}

// Prints message in red
func PrintError(message string) {
	fmt.Println(Red + message + Reset)
}

// Formats meter point info with appropriate colors
func FormatMeterPoint(id, supplier, consumer, address, postcode, city string, index int) string {
	return fmt.Sprintf("%d) %s\n Supplier: %s\n Consumer: %s\n Address: %s, %s %s",
		index+1,
		Cyan+id+Reset, // ID in cyan
		supplier,
		consumer,
		address,
		postcode,
		city,
	)
}

// Displays pre-formatted options and gets user selection
func GetUserChoice(title string, formattedOptions []string) int {
	reader := bufio.NewReader(os.Stdin)

	PrintInfo(fmt.Sprintf("\n%s\n", title))
	for _, option := range formattedOptions {
		fmt.Println(option)
		fmt.Println() // Add blank line between options
	}

	maxChoice := len(formattedOptions)

	for {
		fmt.Printf("Please select (1-%d): ", maxChoice)
		input, err := reader.ReadString('\n')
		if err != nil {
			PrintError("Error reading input, please try again.")
			continue
		}

		input = strings.TrimSpace(input)
		choice, err := strconv.Atoi(input)
		if err != nil {
			PrintError("Please enter a valid number.")
			continue
		}

		if choice < 1 || choice > maxChoice {
			PrintError(fmt.Sprintf("Please enter a number between 1 and %d.", maxChoice))
			continue
		}

		return choice - 1 // Return 0-indexed
	}
}

// Displays simple text options
func GetSimpleChoice(title string, options []string) int {
	reader := bufio.NewReader(os.Stdin)

	fmt.Printf("\n%s\n", title)
	for i, option := range options {
		fmt.Printf("%d) %s\n", i+1, option)
	}

	maxChoice := len(options)

	for {
		fmt.Printf("Please select (1-%d): ", maxChoice)
		input, err := reader.ReadString('\n')
		if err != nil {
			PrintError("Error reading input, please try again.")
			continue
		}

		input = strings.TrimSpace(input)
		choice, err := strconv.Atoi(input)
		if err != nil {
			PrintError("Please enter a valid number.")
			continue
		}

		if choice < 1 || choice > maxChoice {
			PrintError(fmt.Sprintf("Please enter a number between 1 and %d.", maxChoice))
			continue
		}

		return choice - 1 // Return 0-indexed
	}
}

// GetUserInput gets string input from user with prompt
func GetUserInput(prompt string) string {
	reader := bufio.NewReader(os.Stdin)

	fmt.Printf("%s: ", prompt)
	input, err := reader.ReadString('\n')
	if err != nil {
		PrintError("Error reading input.")
		return ""
	}

	return strings.TrimSpace(input)
}
