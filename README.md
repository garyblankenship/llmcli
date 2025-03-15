# GGUF (Groovy GGUF Utility Functions)

GGUF is a powerful Go application for managing and interacting with large language models using llama.cpp. It provides a comprehensive set of functions for downloading, running, and chatting with AI models, as well as managing a local database of model information.

## 🚀 Features

- Download models from Hugging Face
- Run models and start server instances
- Interactive chat sessions with models
- Manage a local database of model information
- Generate embeddings
- Tokenize and detokenize text
- Monitor running model servers
- Fetch recent and trending GGUF models from Hugging Face

![gguf](gguf-old/gguf.png)

## 📋 Prerequisites

Before you begin, ensure you have the following installed:

- `llama-server` command (macOS: `brew install llama.cpp`)
- `huggingface-cli` command (macOS: `brew install huggingface-cli`)
- `sqlite3` (usually pre-installed on macOS)
- Go 1.21 or newer

## 🛠 Installation

1. Clone this repository:
   ```
   git clone https://github.com/yourusername/gguf.git
   cd gguf
   ```

2. Build the application:
   ```
   go build -o gguf cmd/gguf/main.go
   ```

3. Optionally, add the binary to your PATH for easier access.

## 🎮 Usage

Here are some common commands:

```bash
# Download a new model
./gguf pull bartowski/Qwen2.5-Math-1.5B-Instruct-GGUF

# List all models
./gguf ls

# Start a chat session with a model
./gguf chat model-slug

# Generate embeddings
./gguf embed model-slug "Your text here"

# Check server health
./gguf health

# Show running processes
./gguf ps

# Get recent GGUF models from Hugging Face
./gguf recent

# Get trending GGUF models from Hugging Face
./gguf trending
```

For a full list of commands, run:

```bash
./gguf
```

For help with a specific command, use:

```bash
./gguf <command> --help
```

## 📜 License

This project is licensed under the MIT License - see the [LICENSE](gguf-old/LICENSE) file for details.

## 🙏 Acknowledgements

- [llama.cpp](https://github.com/ggerganov/llama.cpp) for the underlying model server
- [Hugging Face](https://huggingface.co/) for hosting the models# llmcli
