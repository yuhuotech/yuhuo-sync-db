package ui

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// ConfirmContinue 询问用户是否继续
func ConfirmContinue(prompt string) bool {
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Printf("%s (y/n): ", prompt)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		input = strings.ToLower(input)

		if input == "y" || input == "yes" {
			return true
		} else if input == "n" || input == "no" {
			return false
		} else {
			fmt.Println("Please enter 'y' or 'n'")
		}
	}
}

// AskForAction 询问用户采取什么操作
func AskForAction(prompt string, options []string) string {
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Printf("%s\n", prompt)
		for i, opt := range options {
			fmt.Printf("%d. %s\n", i+1, opt)
		}
		fmt.Print("Enter your choice (1-" + fmt.Sprintf("%d", len(options)) + "): ")

		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		// 验证输入
		for i, opt := range options {
			if input == fmt.Sprintf("%d", i+1) || input == opt {
				return opt
			}
		}

		fmt.Println("Invalid choice, please try again")
	}
}
