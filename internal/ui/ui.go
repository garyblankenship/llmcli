package ui

import (
	"fmt"
)

// Color constants
const (
	colorReset   = "\033[0m"
	colorCyan    = "\033[0;36m"
	colorGreen   = "\033[0;32m"
	colorYellow  = "\033[0;33m"
	colorMagenta = "\033[0;35m"
	colorGray    = "\033[0;90m"
)

// PrintInfo prints an info message
func PrintInfo(msg string) {
	fmt.Printf("%s[INFO]%s %s\n", colorGreen, colorReset, msg)
}

// PrintWarn prints a warning message
func PrintWarn(msg string) {
	fmt.Printf("%s[WARN]%s %s\n", colorYellow, colorReset, msg)
}

// PrintError prints an error message
func PrintError(msg string) {
	fmt.Printf("%s[ERROR]%s %s\n", "\033[0;31m", colorReset, msg)
}

// PrintHelp prints help for a command
func PrintHelp(command, description, args string) {
	fmt.Printf("Usage: llm-cli %s%s%s %s\n", colorGreen, command, colorReset, args)
	fmt.Println(description)
	fmt.Println()
	
	if args != "" {
		fmt.Println("Arguments:")
		fmt.Printf("  %s\n", args)
	}
}

// PrintUsage prints the usage information
func PrintUsage() {
	fmt.Printf("%sUsage:%s llm-cli %s<command>%s [options]\n\n", colorCyan, colorReset, colorGreen, colorReset)

	fmt.Printf("%sModel Management:%s\n", colorYellow, colorReset)
	printCommand("pull <model_id>", "Download a new model")
	printCommand("rm <slug>", "Remove a model")
	printCommand("ls", "List all models")
	printCommand("alias <old> <new>", "Create an alias for a model")
	printCommand("import", "Import existing models")
	fmt.Println()

	fmt.Printf("%sModel Operations:%s\n", colorYellow, colorReset)
	printCommand("run <slug> [text]", "Run a model server and optionally complete text")
	printCommand("chat <slug>", "Start a chat session")
	printCommand("embed <slug> <text>", "Generate embeddings")
	printCommand("tokenize <slug> <text>", "Tokenize text")
	printCommand("detokenize <slug> <tokens>", "Detokenize text")
	fmt.Println()

	fmt.Printf("%sServer Information:%s\n", colorYellow, colorReset)
	printCommand("health", "Check server health")
	printCommand("props", "Get server properties")
	printCommand("ps", "Show running processes")
	printCommand("kill <slug|all>", "Kill a model server")
	printCommand("reset", "Reset the database")
	printCommand("recent", "Get most recent GGUF models")
	printCommand("trending", "Get trending GGUF models")
	fmt.Println()

	fmt.Printf("%sFor more information, use:%s llm-cli %s<command> --help%s\n", 
		colorMagenta, colorReset, colorGreen, colorReset)
}

// printCommand prints a formatted command with description
func printCommand(cmd, desc string) {
	fmt.Printf("  %s%-26s%s %s%s%s %s\n", colorGreen, cmd, colorReset, 
		colorGray, ".....................", colorReset, desc)
}