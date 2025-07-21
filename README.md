# Bulker

A parallel runner for security tools with automatic optimisations.

## Usage

**Build**
```bash
git clone https://github.com/aleister1102/bulker.git
cd bulker
go build -o bulker # or bulker.exe for Windows
```
*Binaries are also attached to GitHub releases.*

**Run**
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
