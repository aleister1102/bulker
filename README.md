# Bulker

A parallel runner for security tools with automatic optimisations.

## Installation

### Download Pre-built Binaries
Download from [GitHub Releases](https://github.com/aleister1102/bulker/releases):

**Linux:**
```bash
# Download and install
curl -L -o bulker https://github.com/aleister1102/bulker/releases/latest/download/bulker-linux-amd64
chmod +x bulker
sudo mv bulker /usr/local/bin/  # Optional: add to PATH
```

**Windows:**
```powershell
# Download bulker-windows-amd64.exe from releases page
# Or use PowerShell:
Invoke-WebRequest -Uri "https://github.com/aleister1102/bulker/releases/latest/download/bulker-windows-amd64.exe" -OutFile "bulker.exe"
```

**macOS:**
```bash
# Download and install
curl -L -o bulker https://github.com/aleister1102/bulker/releases/latest/download/bulker-darwin-amd64
chmod +x bulker
sudo mv bulker /usr/local/bin/  # Optional: add to PATH
```

### Build from Source
```bash
git clone https://github.com/aleister1102/bulker.git
cd bulker
go build -o bulker  # or bulker.exe for Windows

# Or use build scripts for cross-platform:
./build.sh         # Linux/macOS
.\build.bat        # Windows
```

## Usage

```bash
# List all tools from config.toml
bulker list

# Run a tool (e.g., httpx)
bulker run httpx -i domains.txt -o httpx_out.txt -t 8 -- -sc -title
```

## Common Flags

| Flag           | Description                          |
|----------------|--------------------------------------|
| `-i, --input`  | Input file (required)                |
| `-o, --output` | Output file                          |
| `-t, --threads`| Parallel threads (default 4)         |
| `-w, --wordlist`| Wordlist for tools like ffuf       |
| `-e, --extra-args`| Extra flags for the wrapped tool   |

## Tools

Bulker reads tool definitions from `config.toml`. See the file for a full list of supported tools and to add your own. 
