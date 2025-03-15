package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/vampire/gguf/internal/config"
	"github.com/vampire/gguf/internal/db"
	"github.com/vampire/gguf/internal/model"
	"github.com/vampire/gguf/internal/server"
	"github.com/vampire/gguf/internal/ui"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	store, err := db.New(cfg.DBPath)
	if err != nil {
		return fmt.Errorf("initializing database: %w", err)
	}
	defer store.Close()

	if len(os.Args) < 2 {
		ui.PrintUsage()
		return nil
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "pull":
		if len(args) < 1 {
			return fmt.Errorf("pull requires a model ID")
		}
		if args[0] == "--help" {
			ui.PrintHelp("pull", "Download a new model from Hugging Face.", "<model_id>")
			return nil
		}
		return model.Pull(store, cfg, args[0])

	case "ls":
		return model.List(store)

	case "rm":
		if len(args) < 1 {
			return fmt.Errorf("rm requires a model slug")
		}
		if args[0] == "--help" {
			ui.PrintHelp("rm", "Remove a model from the filesystem and database.", "<slug>")
			return nil
		}
		return model.Remove(store, cfg, args[0])

	case "alias":
		if len(args) < 2 {
			return fmt.Errorf("alias requires old and new slugs")
		}
		if args[0] == "--help" {
			ui.PrintHelp("alias", "Create an alias for a model.", "<old_slug> <new_slug>")
			return nil
		}
		return model.Alias(store, args[0], args[1])

	case "import":
		if len(args) > 0 && args[0] == "--help" {
			ui.PrintHelp("import", "Import existing models from the filesystem into the database.", "")
			return nil
		}
		return model.ImportExisting(store, cfg)

	case "reset":
		if len(args) > 0 && args[0] == "--help" {
			ui.PrintHelp("reset", "Reset the database and re-import existing models.", "")
			return nil
		}
		return model.ResetDB(store, cfg)

	case "run":
		if len(args) < 1 {
			return fmt.Errorf("run requires a model slug")
		}
		if args[0] == "--help" {
			ui.PrintHelp("run", "Run a model server and optionally complete text.", "<slug> [text]")
			return nil
		}
		slug := args[0]
		text := strings.Join(args[1:], " ")
		return server.Run(store, cfg, slug, text)

	case "chat":
		if len(args) < 1 {
			return fmt.Errorf("chat requires a model slug")
		}
		if args[0] == "--help" {
			ui.PrintHelp("chat", "Start a chat session with the specified model.", "<slug>")
			return nil
		}
		return server.Chat(store, cfg, args[0])

	case "embed":
		if len(args) < 2 {
			return fmt.Errorf("embed requires a model slug and text")
		}
		if args[0] == "--help" {
			ui.PrintHelp("embed", "Generate embeddings for the given text.", "<slug> <text>")
			return nil
		}
		return server.Embed(store, cfg, args[0], strings.Join(args[1:], " "))

	case "tokenize":
		if len(args) < 2 {
			return fmt.Errorf("tokenize requires a model slug and text")
		}
		if args[0] == "--help" {
			ui.PrintHelp("tokenize", "Tokenize text using the specified model.", "<slug> <text>")
			return nil
		}
		return server.Tokenize(store, cfg, args[0], strings.Join(args[1:], " "))

	case "detokenize":
		if len(args) < 2 {
			return fmt.Errorf("detokenize requires a model slug and tokens")
		}
		if args[0] == "--help" {
			ui.PrintHelp("detokenize", "Detokenize tokens using the specified model.", "<slug> <tokens>")
			return nil
		}
		return server.Detokenize(store, cfg, args[0], args[1])

	case "health":
		if len(args) > 0 && args[0] == "--help" {
			ui.PrintHelp("health", "Check the health status of the running server.", "")
			return nil
		}
		return server.CheckHealth(cfg)

	case "props":
		if len(args) > 0 && args[0] == "--help" {
			ui.PrintHelp("props", "Get the properties of the running server.", "")
			return nil
		}
		return server.GetProperties(cfg)

	case "ps":
		if len(args) > 0 && args[0] == "--help" {
			ui.PrintHelp("ps", "Show running llama-server processes.", "")
			return nil
		}
		return server.ListProcesses(store)

	case "kill":
		if len(args) < 1 {
			return fmt.Errorf("kill requires a model slug or 'all'")
		}
		if args[0] == "--help" {
			ui.PrintHelp("kill", "Kill a model server or all servers.", "<slug|all>")
			return nil
		}

		if args[0] == "all" {
			return server.KillAll()
		}
		return server.Kill(args[0])

	case "recent":
		if len(args) > 0 && args[0] == "--help" {
			ui.PrintHelp("recent", "Get the 20 most recent GGUF models from Hugging Face.", "")
			return nil
		}
		return model.GetRecent()

	case "trending":
		if len(args) > 0 && args[0] == "--help" {
			ui.PrintHelp("trending", "Get trending GGUF models from Hugging Face.", "")
			return nil
		}
		return model.GetTrending()

	default:
		ui.PrintUsage()
		return fmt.Errorf("unknown command: %s", cmd)
	}
}