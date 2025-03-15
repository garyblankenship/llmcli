package model

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/garyblankenship/llmcli/internal/config"
	"github.com/garyblankenship/llmcli/internal/db"
	"github.com/garyblankenship/llmcli/internal/ui"
)

// huggingFaceModel represents a model from the Hugging Face API
type huggingFaceModel struct {
	ModelID      string   `json:"modelId"`
	LastModified string   `json:"lastModified"`
	Tags         []string `json:"tags"`
	Siblings     []struct {
		RFileName string `json:"rfilename"`
	} `json:"siblings"`
	Downloads int `json:"downloads,omitempty"`
	Likes     int `json:"likes,omitempty"`
}

// validateModelID checks if a model ID is valid (author/model-name format)
func validateModelID(modelID string) bool {
	pattern := `^[a-zA-Z0-9_-]+/[a-zA-Z0-9_.-]+$`
	matched, _ := regexp.MatchString(pattern, modelID)
	return matched
}

// generateSlug creates a slug from a model ID
func generateSlug(modelID string) string {
	// Convert to lowercase
	slug := strings.ToLower(modelID)
	
	// Replace slashes with hyphens
	slug = strings.ReplaceAll(slug, "/", "-")
	
	// Remove any characters that aren't alphanumeric or hyphens
	re := regexp.MustCompile(`[^a-z0-9-]`)
	slug = re.ReplaceAllString(slug, "-")
	
	// Remove leading and trailing hyphens
	slug = strings.Trim(slug, "-")
	
	return slug
}

// Pull downloads a model from Hugging Face
func Pull(store *db.Store, cfg *config.Config, modelID string) error {
	if !validateModelID(modelID) {
		return fmt.Errorf("invalid model ID format: %s", modelID)
	}

	// Create model directory
	modelDir := filepath.Join(cfg.ModelsDir, modelID)
	
	// Check if model already exists
	if _, err := os.Stat(modelDir); err == nil {
		// Directory exists, check for .gguf files
		files, err := filepath.Glob(filepath.Join(modelDir, "*.gguf"))
		if err != nil {
			return fmt.Errorf("checking existing files: %w", err)
		}
		
		if len(files) > 0 {
			ui.PrintWarn(fmt.Sprintf("Model already exists in %s. Remove existing files to re-download.", modelDir))
			return nil
		}
	}
	
	// Fetch model information from Hugging Face API
	ui.PrintInfo(fmt.Sprintf("Fetching model information for %s...", modelID))
	apiURL := fmt.Sprintf("https://huggingface.co/api/models/%s?filter=gguf&sort=lastModified", modelID)
	
	resp, err := http.Get(apiURL)
	if err != nil {
		return fmt.Errorf("fetching model information: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API returned status %d", resp.StatusCode)
	}
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading API response: %w", err)
	}
	
	var modelInfo huggingFaceModel
	if err := json.Unmarshal(body, &modelInfo); err != nil {
		return fmt.Errorf("parsing model information: %w", err)
	}
	
	// Find q4_k_m.gguf file to download
	var fileToDownload string
	for _, sibling := range modelInfo.Siblings {
		lowerName := strings.ToLower(sibling.RFileName)
		if strings.HasSuffix(lowerName, "q4_k_m.gguf") {
			fileToDownload = sibling.RFileName
			break
		}
	}
	
	if fileToDownload == "" {
		return fmt.Errorf("no q4_k_m.gguf file found for %s", modelID)
	}
	
	// Create directory if it doesn't exist
	if err := os.MkdirAll(modelDir, 0755); err != nil {
		return fmt.Errorf("creating model directory: %w", err)
	}
	
	// Download the file using huggingface-cli
	ui.PrintInfo(fmt.Sprintf("Downloading %s for model %s...", fileToDownload, modelID))
	cmd := exec.Command("huggingface-cli", "download", modelID, fileToDownload, "--local-dir", modelDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("downloading model: %w", err)
	}
	
	downloadedFile := filepath.Join(modelDir, fileToDownload)
	if _, err := os.Stat(downloadedFile); err != nil {
		return fmt.Errorf("downloaded file not found: %w", err)
	}
	
	// Get file size
	fileInfo, err := os.Stat(downloadedFile)
	if err != nil {
		return fmt.Errorf("getting file info: %w", err)
	}
	
	fileSize := fmt.Sprintf("%dM", fileInfo.Size()/(1024*1024)) // Size in MB
	
	// Generate slug
	slug := generateSlug(modelID)
	
	// Add to database
	if err := store.AddModel(slug, modelID, fileToDownload, downloadedFile, fileSize); err != nil {
		return fmt.Errorf("adding model to database: %w", err)
	}
	
	ui.PrintInfo(fmt.Sprintf("Model added to database with slug: %s", slug))
	fmt.Printf("To use this model, run: llm-cli chat %s\n", slug)
	
	return nil
}

// List displays all models
func List(store *db.Store) error {
	models, err := store.GetAllModels()
	if err != nil {
		return fmt.Errorf("retrieving models: %w", err)
	}
	
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "SLUG\tMODEL ID\tSIZE\tLAST USED")
	
	for _, model := range models {
		lastUsed := "Never"
		if model.LastUsed.Valid {
			lastUsed = model.LastUsed.Time.Format("2006-01-02 15:04:05")
		}
		
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", 
			model.Slug, model.ModelID, model.FileSize, lastUsed)
	}
	
	return w.Flush()
}

// Remove removes a model
func Remove(store *db.Store, cfg *config.Config, slug string) error {
	model, err := store.GetModelBySlug(slug)
	if err != nil {
		return err
	}
	
	// Remove file
	if err := os.Remove(model.FilePath); err != nil {
		return fmt.Errorf("removing file: %w", err)
	}
	
	// Remove from database
	if err := store.RemoveModel(slug); err != nil {
		return err
	}
	
	ui.PrintInfo(fmt.Sprintf("Model '%s' removed from filesystem and database.", slug))
	return nil
}

// Alias creates an alias for a model
func Alias(store *db.Store, oldSlug, newSlug string) error {
	// Check if old slug exists
	if _, err := store.GetModelBySlug(oldSlug); err != nil {
		return err
	}
	
	// Check if new slug already exists
	if _, err := store.GetModelBySlug(newSlug); err == nil {
		return fmt.Errorf("model with slug '%s' already exists", newSlug)
	}
	
	// Update slug
	if err := store.UpdateModelSlug(oldSlug, newSlug); err != nil {
		return err
	}
	
	ui.PrintInfo(fmt.Sprintf("Model '%s' aliased to '%s'.", oldSlug, newSlug))
	return nil
}

// ImportExisting imports existing models from the filesystem
func ImportExisting(store *db.Store, cfg *config.Config) error {
	ui.PrintInfo(fmt.Sprintf("Scanning for existing models in %s...", cfg.ModelsDir))
	
	err := filepath.Walk(cfg.ModelsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		
		if !info.IsDir() && strings.HasSuffix(strings.ToLower(path), ".gguf") {
			rel, err := filepath.Rel(cfg.ModelsDir, path)
			if err != nil {
				return fmt.Errorf("getting relative path: %w", err)
			}
			
			// Extract model ID from path
			parts := strings.Split(rel, string(os.PathSeparator))
			if len(parts) < 2 {
				return nil // Skip files not in expected directory structure
			}
			
			modelID := parts[0]
			if len(parts) > 2 {
				// Handle nested directories
				modelID = filepath.Join(parts[:len(parts)-1]...)
			}
			
			fileName := filepath.Base(path)
			fileSize := fmt.Sprintf("%dM", info.Size()/(1024*1024)) // Size in MB
			slug := generateSlug(modelID)
			
			// Add to database
			if err := store.AddModel(slug, modelID, fileName, path, fileSize); err != nil {
				ui.PrintWarn(fmt.Sprintf("Failed to import model %s: %v", path, err))
				return nil
			}
			
			ui.PrintInfo(fmt.Sprintf("Imported model: %s", slug))
		}
		
		return nil
	})
	
	if err != nil {
		return fmt.Errorf("walking models directory: %w", err)
	}
	
	ui.PrintInfo("Import completed.")
	return nil
}

// ResetDB resets the database and reimports models
func ResetDB(store *db.Store, cfg *config.Config) error {
	ui.PrintWarn("Resetting the database...")
	
	// Close current connection
	if err := store.Close(); err != nil {
		return fmt.Errorf("closing database: %w", err)
	}
	
	// Remove database file
	if err := os.Remove(cfg.DBPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing database file: %w", err)
	}
	
	// Create new connection
	newStore, err := db.New(cfg.DBPath)
	if err != nil {
		return fmt.Errorf("initializing new database: %w", err)
	}
	
	// Import existing models
	if err := ImportExisting(newStore, cfg); err != nil {
		return fmt.Errorf("importing models: %w", err)
	}
	
	ui.PrintInfo("Database reset and import complete.")
	return nil
}

// GetRecent fetches recent GGUF models from Hugging Face
func GetRecent() error {
	url := "https://huggingface.co/api/models?filter=gguf&sort=lastModified"
	
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("fetching recent models: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API returned status %d", resp.StatusCode)
	}
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading API response: %w", err)
	}
	
	var models []huggingFaceModel
	if err := json.Unmarshal(body, &models); err != nil {
		return fmt.Errorf("parsing models: %w", err)
	}
	
	// Pre-process models to handle any missing fields
	for i := range models {
		if models[i].LastModified == "" {
			models[i].LastModified = "N/A"
		}
	}
	
	// Get terminal width for better formatting
	cmd := exec.Command("stty", "size")
	cmd.Stdin = os.Stdin
	out, err := cmd.Output()
	
	termWidth := 100 // Default width if we can't get actual terminal width
	if err == nil {
		parts := strings.Split(strings.TrimSpace(string(out)), " ")
		if len(parts) >= 2 {
			if width, err := strconv.Atoi(parts[1]); err == nil {
				termWidth = width
			}
		}
	}
	
	// Calculate column widths
	modelIDWidth := termWidth / 2
	if modelIDWidth > 60 {
		modelIDWidth = 60
	}
	
	dateWidth := 20
	likesWidth := 5
	downloadsWidth := 9
	
	// Print header with border
	fmt.Println(strings.Repeat("─", termWidth))
	fmt.Printf("%-*s %-*s %*s %*s\n",
		modelIDWidth, "MODEL ID",
		dateWidth, "LAST MODIFIED",
		likesWidth, "LIKES",
		downloadsWidth, "DOWNLOADS")
	fmt.Println(strings.Repeat("─", termWidth))
	
	// Format and print each model
	count := 0
	for _, model := range models {
		// Check if model has GGUF tag
		hasGGUFTag := false
		for _, tag := range model.Tags {
			if tag == "gguf" {
				hasGGUFTag = true
				break
			}
		}
		
		if hasGGUFTag {
			// Format the date to be more readable
			dateStr := model.LastModified
			if len(dateStr) > 10 {
				dateStr = dateStr[:10] // Just keep YYYY-MM-DD
			}
			
			// Truncate long model IDs
			modelID := model.ModelID
			if len(modelID) > modelIDWidth {
				modelID = modelID[:modelIDWidth-3] + "..."
			}
			
			// Format with colorization
			fmt.Printf("\033[1;36m%-*s\033[0m \033[0;33m%-*s\033[0m %*d %*d\n",
				modelIDWidth, modelID,
				dateWidth, dateStr,
				likesWidth, model.Likes,
				downloadsWidth, model.Downloads)
			
			count++
			if count >= 20 {
				break
			}
		}
	}
	
	fmt.Println(strings.Repeat("─", termWidth))
	fmt.Printf("Showing %d recent GGUF models from Hugging Face\n", count)
	
	return nil
}

// GetTrending fetches trending GGUF models from Hugging Face
func GetTrending() error {
	// Instead of 'trending', we'll sort by downloads which is a more reliable parameter
	url := "https://huggingface.co/api/models?filter=gguf&sort=downloads"
	
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("fetching trending models: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API returned status %d", resp.StatusCode)
	}
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading API response: %w", err)
	}
	
	var models []huggingFaceModel
	if err := json.Unmarshal(body, &models); err != nil {
		return fmt.Errorf("parsing models: %w", err)
	}
	
	// Pre-process models to handle any missing fields
	for i := range models {
		if models[i].LastModified == "" {
			models[i].LastModified = "N/A"
		}
	}
	
	// Get terminal width for better formatting
	cmd := exec.Command("stty", "size")
	cmd.Stdin = os.Stdin
	out, err := cmd.Output()
	
	termWidth := 100 // Default width if we can't get actual terminal width
	if err == nil {
		parts := strings.Split(strings.TrimSpace(string(out)), " ")
		if len(parts) >= 2 {
			if width, err := strconv.Atoi(parts[1]); err == nil {
				termWidth = width
			}
		}
	}
	
	// Calculate column widths
	modelIDWidth := termWidth / 2
	if modelIDWidth > 60 {
		modelIDWidth = 60
	}
	
	dateWidth := 12
	likesWidth := 7
	downloadsWidth := 12
	
	// Print header with border
	fmt.Println(strings.Repeat("─", termWidth))
	fmt.Printf("%-*s %-*s %*s %*s\n",
		modelIDWidth, "MODEL ID",
		dateWidth, "LAST UPDATED",
		likesWidth, "LIKES",
		downloadsWidth, "DOWNLOADS")
	fmt.Println(strings.Repeat("─", termWidth))
	
	// Format and print each model
	count := 0
	for _, model := range models {
		// Check if model has GGUF tag
		hasGGUFTag := false
		for _, tag := range model.Tags {
			if tag == "gguf" {
				hasGGUFTag = true
				break
			}
		}
		
		if hasGGUFTag {
			// Format the date to be more readable
			dateStr := model.LastModified
			if len(dateStr) > 10 {
				dateStr = dateStr[:10] // Just keep YYYY-MM-DD
			}
			
			// Truncate long model IDs
			modelID := model.ModelID
			if len(modelID) > modelIDWidth {
				modelID = modelID[:modelIDWidth-3] + "..."
			}
			
			// Add colors based on popularity
			likesColor := "\033[0m"     // Default color
			if model.Likes > 100 {
				likesColor = "\033[1;33m" // Yellow for popular
			}
			if model.Likes > 500 {
				likesColor = "\033[1;32m" // Green for very popular
			}
			
			downloadsColor := "\033[0m"
			if model.Downloads > 1000 {
				downloadsColor = "\033[1;33m"
			}
			if model.Downloads > 10000 {
				downloadsColor = "\033[1;32m"
			}
			
			// Format with colorization
			fmt.Printf("\033[1;36m%-*s\033[0m \033[0;33m%-*s\033[0m %s%*d\033[0m %s%*d\033[0m\n",
				modelIDWidth, modelID,
				dateWidth, dateStr,
				likesColor, likesWidth, model.Likes,
				downloadsColor, downloadsWidth, model.Downloads)
			
			count++
			if count >= 20 {
				break
			}
		}
	}
	
	fmt.Println(strings.Repeat("─", termWidth))
	fmt.Printf("Showing the top %d trending GGUF models from Hugging Face\n", count)
	
	return nil
}