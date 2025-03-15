package server

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/vampire/gguf/internal/config"
	"github.com/vampire/gguf/internal/db"
	"github.com/vampire/gguf/internal/ui"
)

// Request types
type completionRequest struct {
	Prompt      string  `json:"prompt"`
	NPredict    int     `json:"n_predict"`
	Temperature float64 `json:"temperature"`
	TopK        int     `json:"top_k"`
	TopP        float64 `json:"top_p"`
	CachePrompt bool    `json:"cache_prompt,omitempty"`
	Stop        []string `json:"stop,omitempty"`
	Stream      bool    `json:"stream,omitempty"`
}

type embeddingRequest struct {
	Content string `json:"content"`
}

type tokenizeRequest struct {
	Content string `json:"content"`
}

// EnsureServerRunning makes sure a server is running for the given model
func EnsureServerRunning(store *db.Store, cfg *config.Config, slug string) error {
	// Get model from database
	model, err := store.GetModelBySlug(slug)
	if err != nil {
		return err
	}

	// Update last used timestamp
	if err := store.UpdateModelLastUsed(slug); err != nil {
		return fmt.Errorf("updating last used timestamp: %w", err)
	}

	// Check if server is already running
	serverRunning, err := IsServerRunningForPath(model.FilePath)
	if err != nil {
		return fmt.Errorf("checking server status: %w", err)
	}

	if serverRunning {
		ui.PrintInfo(fmt.Sprintf("Server for model %s is already running.", slug))
		return nil
	}

	// Start server
	ui.PrintInfo(fmt.Sprintf("Starting server for model %s...", slug))
	logFile := fmt.Sprintf("/tmp/llama_server_%s.log", slug)

	cmd := exec.Command(cfg.LlamaServer, "-m", model.FilePath, "--port", strconv.Itoa(cfg.DefaultPort))
	stdout, err := os.Create(logFile)
	if err != nil {
		return fmt.Errorf("creating log file: %w", err)
	}
	defer stdout.Close()

	cmd.Stdout = stdout
	cmd.Stderr = stdout

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting server: %w", err)
	}

	ui.PrintInfo(fmt.Sprintf("Server started with PID %d. Logs: %s", cmd.Process.Pid, logFile))

	// Wait for server to be ready
	if err := WaitForServer(cfg.DefaultPort, 300); err != nil {
		return fmt.Errorf("waiting for server: %w", err)
	}

	return nil
}

// IsServerRunningForPath checks if a server is running for the given model path
func IsServerRunningForPath(modelPath string) (bool, error) {
	cmd := exec.Command("pgrep", "-f", fmt.Sprintf("llama-server.*%s", modelPath))
	output, err := cmd.Output()
	
	if err != nil {
		// pgrep returns error when no process is found, which is not an error for us
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return false, nil
		}
		return false, fmt.Errorf("checking server: %w", err)
	}
	
	return len(output) > 0, nil
}

// IsServerRunning checks if a server is running on the given port
func IsServerRunning(port int) (bool, error) {
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/health", port))
	if err != nil {
		return false, nil
	}
	defer resp.Body.Close()
	
	return resp.StatusCode == http.StatusOK, nil
}

// WaitForServer waits for the server to be ready
func WaitForServer(port, maxWaitSeconds int) error {
	ui.PrintInfo("Waiting for server to be ready...")
	
	for i := 0; i < maxWaitSeconds; i++ {
		if i > 0 && i%10 == 0 {
			fmt.Print(".")
		}
		
		running, _ := IsServerRunning(port)
		if running {
			fmt.Println() // End the dots with a newline
			ui.PrintInfo(fmt.Sprintf("Server is ready after %d seconds.", i))
			return nil
		}
		
		time.Sleep(1 * time.Second)
	}
	
	return fmt.Errorf("server failed to start within %d seconds", maxWaitSeconds)
}

// Run starts a model server and optionally completes text
func Run(store *db.Store, cfg *config.Config, slug, text string) error {
	if err := EnsureServerRunning(store, cfg, slug); err != nil {
		return err
	}
	
	if text == "" {
		ui.PrintInfo(fmt.Sprintf("Server for model %s is running. Use 'gguf chat %s' to start a chat session.", slug, slug))
		return nil
	}
	
	// Complete text
	ui.PrintInfo(fmt.Sprintf("Completing text: %s", text))
	
	// Prepare request
	req := completionRequest{
		Prompt:      text,
		NPredict:    cfg.NPredictMax,
		Temperature: cfg.Temperature,
		TopK:        cfg.TopK,
		TopP:        cfg.TopP,
	}
	
	reqBody, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshaling request: %w", err)
	}
	
	// Send request
	resp, err := http.Post(fmt.Sprintf("%s/completion", cfg.APIURL), "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API returned status %d: %s", resp.StatusCode, body)
	}
	
	// Parse response
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("parsing response: %w", err)
	}
	
	// Print response
	fmt.Println(strings.Repeat("─", 80))
	
	if content, ok := result["content"].(string); ok {
		fmt.Println(content)
	}
	
	return nil
}

// Chat starts an interactive chat session
func Chat(store *db.Store, cfg *config.Config, slug string) error {
	if err := EnsureServerRunning(store, cfg, slug); err != nil {
		return err
	}

	ui.PrintInfo("Starting chat session. Type 'exit' to end.")
	
	// Chat history
	var chatHistory []string
	
	reader := bufio.NewReader(os.Stdin)
	
	for {
		fmt.Print("User: ")
		userInput, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("reading input: %w", err)
		}
		
		userInput = strings.TrimSpace(userInput)
		if userInput == "exit" {
			break
		}
		
		// Add to history
		chatHistory = append(chatHistory, userInput)
		
		// Format prompt with chat history
		prompt := formatChatPrompt(chatHistory)
		
		// Prepare request
		req := completionRequest{
			Prompt:      prompt,
			NPredict:    cfg.NPredictMax,
			Temperature: cfg.Temperature,
			TopK:        cfg.TopK,
			TopP:        cfg.TopP,
			CachePrompt: true,
			Stop:        []string{"\n### Human:"},
			Stream:      true,
		}
		
		reqBody, err := json.Marshal(req)
		if err != nil {
			return fmt.Errorf("marshaling request: %w", err)
		}

		// Create HTTP request
		httpReq, err := http.NewRequest("POST", fmt.Sprintf("%s/completion", cfg.APIURL), bytes.NewBuffer(reqBody))
		if err != nil {
			return fmt.Errorf("creating request: %w", err)
		}
		
		httpReq.Header.Set("Content-Type", "application/json")
		
		// Send request
		client := &http.Client{}
		resp, err := client.Do(httpReq)
		if err != nil {
			return fmt.Errorf("sending request: %w", err)
		}
		
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return fmt.Errorf("API returned status %d: %s", resp.StatusCode, body)
		}
		
		// Stream response
		fmt.Print("Assistant: ")
		var fullResponse strings.Builder
		
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			
			if strings.HasPrefix(line, "data: ") {
				data := strings.TrimPrefix(line, "data: ")
				
				var streamData map[string]interface{}
				if err := json.Unmarshal([]byte(data), &streamData); err != nil {
					continue
				}
				
				if content, ok := streamData["content"].(string); ok {
					fmt.Print(content)
					fullResponse.WriteString(content)
				}
			}
		}
		
		fmt.Println()
		resp.Body.Close()
		
		if err := scanner.Err(); err != nil {
			return fmt.Errorf("reading stream: %w", err)
		}
		
		// Add response to history
		chatHistory = append(chatHistory, fullResponse.String())
	}
	
	ui.PrintInfo("Chat session ended.")
	return nil
}

// formatChatPrompt formats a chat prompt with history
func formatChatPrompt(history []string) string {
	var b strings.Builder
	
	// Instruction
	b.WriteString("A chat between a curious human and an artificial intelligence assistant. ")
	b.WriteString("The assistant gives helpful, detailed, and polite answers to the human's questions.")
	
	// Format history as alternating human/assistant messages
	for i := 0; i < len(history); i += 2 {
		b.WriteString("\n### Human: ")
		b.WriteString(history[i])
		
		if i+1 < len(history) {
			b.WriteString("\n### Assistant: ")
			b.WriteString(history[i+1])
		}
	}
	
	// Add final human message if there's an odd number of messages
	if len(history)%2 == 1 {
		b.WriteString("\n### Assistant: ")
	}
	
	return b.String()
}

// Embed generates embeddings for text
func Embed(store *db.Store, cfg *config.Config, slug, text string) error {
	if err := EnsureServerRunning(store, cfg, slug); err != nil {
		return err
	}
	
	// Prepare request
	req := embeddingRequest{
		Content: text,
	}
	
	reqBody, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshaling request: %w", err)
	}
	
	// Send request
	resp, err := http.Post(fmt.Sprintf("%s/embedding", cfg.APIURL), "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API returned status %d: %s", resp.StatusCode, body)
	}
	
	// Parse and print response
	var prettyJSON bytes.Buffer
	decoder := json.NewDecoder(resp.Body)
	encoder := json.NewEncoder(&prettyJSON)
	encoder.SetIndent("", "  ")
	
	var value interface{}
	if err := decoder.Decode(&value); err != nil {
		return fmt.Errorf("parsing response: %w", err)
	}
	
	if err := encoder.Encode(value); err != nil {
		return fmt.Errorf("formatting response: %w", err)
	}
	
	fmt.Println(prettyJSON.String())
	return nil
}

// Tokenize tokenizes text
func Tokenize(store *db.Store, cfg *config.Config, slug, text string) error {
	if err := EnsureServerRunning(store, cfg, slug); err != nil {
		return err
	}
	
	// Prepare request
	req := tokenizeRequest{
		Content: text,
	}
	
	reqBody, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshaling request: %w", err)
	}
	
	// Send request
	resp, err := http.Post(fmt.Sprintf("%s/tokenize", cfg.APIURL), "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API returned status %d: %s", resp.StatusCode, body)
	}
	
	// Parse and print response
	var prettyJSON bytes.Buffer
	decoder := json.NewDecoder(resp.Body)
	encoder := json.NewEncoder(&prettyJSON)
	encoder.SetIndent("", "  ")
	
	var value interface{}
	if err := decoder.Decode(&value); err != nil {
		return fmt.Errorf("parsing response: %w", err)
	}
	
	if err := encoder.Encode(value); err != nil {
		return fmt.Errorf("formatting response: %w", err)
	}
	
	fmt.Println(prettyJSON.String())
	return nil
}

// Detokenize detokenizes tokens
func Detokenize(store *db.Store, cfg *config.Config, slug, tokensStr string) error {
	if err := EnsureServerRunning(store, cfg, slug); err != nil {
		return err
	}
	
	// Parse tokens string as JSON array
	var tokens []int
	if err := json.Unmarshal([]byte(tokensStr), &tokens); err != nil {
		return fmt.Errorf("parsing tokens: %w", err)
	}
	
	// Prepare request
	reqBody, err := json.Marshal(map[string]interface{}{
		"tokens": tokens,
	})
	if err != nil {
		return fmt.Errorf("marshaling request: %w", err)
	}
	
	// Send request
	resp, err := http.Post(fmt.Sprintf("%s/detokenize", cfg.APIURL), "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API returned status %d: %s", resp.StatusCode, body)
	}
	
	// Parse and print response
	var prettyJSON bytes.Buffer
	decoder := json.NewDecoder(resp.Body)
	encoder := json.NewEncoder(&prettyJSON)
	encoder.SetIndent("", "  ")
	
	var value interface{}
	if err := decoder.Decode(&value); err != nil {
		return fmt.Errorf("parsing response: %w", err)
	}
	
	if err := encoder.Encode(value); err != nil {
		return fmt.Errorf("formatting response: %w", err)
	}
	
	fmt.Println(prettyJSON.String())
	return nil
}

// CheckHealth checks the server health
func CheckHealth(cfg *config.Config) error {
	// Send request
	resp, err := http.Get(fmt.Sprintf("%s/health", cfg.APIURL))
	if err != nil {
		return fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API returned status %d: %s", resp.StatusCode, body)
	}
	
	// Parse and print response
	var prettyJSON bytes.Buffer
	decoder := json.NewDecoder(resp.Body)
	encoder := json.NewEncoder(&prettyJSON)
	encoder.SetIndent("", "  ")
	
	var value interface{}
	if err := decoder.Decode(&value); err != nil {
		return fmt.Errorf("parsing response: %w", err)
	}
	
	if err := encoder.Encode(value); err != nil {
		return fmt.Errorf("formatting response: %w", err)
	}
	
	ui.PrintInfo("Server is healthy.")
	fmt.Println(prettyJSON.String())
	
	return nil
}

// GetProperties gets the server properties
func GetProperties(cfg *config.Config) error {
	// Send request
	resp, err := http.Get(fmt.Sprintf("%s/props", cfg.APIURL))
	if err != nil {
		return fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API returned status %d: %s", resp.StatusCode, body)
	}
	
	// Parse and print response
	var prettyJSON bytes.Buffer
	decoder := json.NewDecoder(resp.Body)
	encoder := json.NewEncoder(&prettyJSON)
	encoder.SetIndent("", "  ")
	
	var value interface{}
	if err := decoder.Decode(&value); err != nil {
		return fmt.Errorf("parsing response: %w", err)
	}
	
	if err := encoder.Encode(value); err != nil {
		return fmt.Errorf("formatting response: %w", err)
	}
	
	fmt.Println(prettyJSON.String())
	
	return nil
}

// ListProcesses lists running llama-server processes
func ListProcesses(store *db.Store) error {
	// Run ps command to get processes
	cmd := exec.Command("ps", "aux")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("running ps command: %w", err)
	}
	
	// Filter for llama-server processes
	var serverProcesses [][]string
	
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.Contains(line, "llama-server") {
			continue
		}
		
		fields := strings.Fields(line)
		if len(fields) < 11 {
			continue
		}
		
		pid := fields[1]
		
		// Extract model file path
		cmdLine := strings.Join(fields[10:], " ")
		parts := strings.Split(cmdLine, "-m ")
		if len(parts) < 2 {
			continue
		}
		
		modelPathParts := strings.Split(parts[1], " ")
		if len(modelPathParts) < 1 {
			continue
		}
		
		modelPath := modelPathParts[0]
		if strings.HasPrefix(modelPath, "\"") && strings.HasSuffix(modelPath, "\"") {
			modelPath = modelPath[1 : len(modelPath)-1]
		}
		
		fileName := filepath.Base(modelPath)
		modelName := strings.TrimSuffix(fileName, filepath.Ext(fileName))
		
		// Look up slug in database
		var slug string
		models, err := store.GetAllModels()
		if err == nil {
			for _, model := range models {
				if strings.HasSuffix(model.FilePath, fileName) {
					slug = model.Slug
					break
				}
			}
		}
		
		if slug == "" {
			slug = "unknown"
		}
		
		serverProcesses = append(serverProcesses, []string{pid, slug, modelName})
	}
	
	if len(serverProcesses) == 0 {
		fmt.Println("No running llama-server processes found.")
		return nil
	}
	
	// Print processes
	fmt.Println("PID\tSLUG\tMODEL")
	for _, proc := range serverProcesses {
		fmt.Printf("%s\t%s\t%s\n", proc[0], proc[1], proc[2])
	}
	
	return nil
}

// Kill terminates a server process
func Kill(target string) error {
	// Check if target is a PID
	if pid, err := strconv.Atoi(target); err == nil {
		// Kill by PID
		process, err := os.FindProcess(pid)
		if err != nil {
			return fmt.Errorf("finding process: %w", err)
		}
		
		if err := process.Signal(syscall.SIGTERM); err != nil {
			return fmt.Errorf("terminating process: %w", err)
		}
		
		ui.PrintInfo(fmt.Sprintf("Process with PID %d terminated.", pid))
		return nil
	}
	
	// Otherwise, treat as a slug and find matching processes
	cmd := exec.Command("pgrep", "-f", fmt.Sprintf("llama-server.*%s", target))
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return fmt.Errorf("no running server found for model '%s'", target)
		}
		return fmt.Errorf("finding processes: %w", err)
	}
	
	pids := strings.Fields(string(output))
	if len(pids) == 0 {
		return fmt.Errorf("no running server found for model '%s'", target)
	}
	
	for _, pidStr := range pids {
		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			continue
		}
		
		process, err := os.FindProcess(pid)
		if err != nil {
			ui.PrintWarn(fmt.Sprintf("Could not find process %d: %v", pid, err))
			continue
		}
		
		if err := process.Signal(syscall.SIGTERM); err != nil {
			ui.PrintError(fmt.Sprintf("Failed to terminate process %d: %v", pid, err))
			continue
		}
		
		ui.PrintInfo(fmt.Sprintf("Server for model '%s' (PID: %d) terminated.", target, pid))
	}
	
	return nil
}

// KillAll terminates all llama-server processes
func KillAll() error {
	// Find all llama-server processes
	cmd := exec.Command("pgrep", "-f", "llama-server")
	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			ui.PrintWarn("No running llama-server processes found.")
			return nil
		}
		return fmt.Errorf("finding processes: %w", err)
	}
	
	pids := strings.Fields(string(output))
	if len(pids) == 0 {
		ui.PrintWarn("No running llama-server processes found.")
		return nil
	}
	
	// Kill each process
	ui.PrintInfo("Killing all llama-server processes...")
	
	for _, pidStr := range pids {
		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			continue
		}
		
		process, err := os.FindProcess(pid)
		if err != nil {
			ui.PrintWarn(fmt.Sprintf("Could not find process %d: %v", pid, err))
			continue
		}
		
		if err := process.Signal(syscall.SIGTERM); err != nil {
			ui.PrintError(fmt.Sprintf("Failed to terminate process %d: %v", pid, err))
		}
	}
	
	// Wait a bit for processes to terminate
	time.Sleep(2 * time.Second)
	
	// Check for any remaining processes and force kill them
	cmd = exec.Command("pgrep", "-f", "llama-server")
	output, err = cmd.Output()
	if err == nil && len(output) > 0 {
		ui.PrintWarn("Some processes didn't terminate cleanly. Force killing...")
		
		pids = strings.Fields(string(output))
		for _, pidStr := range pids {
			pid, err := strconv.Atoi(pidStr)
			if err != nil {
				continue
			}
			
			process, err := os.FindProcess(pid)
			if err != nil {
				continue
			}
			
			if err := process.Signal(syscall.SIGKILL); err != nil {
				ui.PrintError(fmt.Sprintf("Failed to force kill process %d: %v", pid, err))
			}
		}
	}
	
	ui.PrintInfo("All llama-server processes terminated.")
	return nil
}