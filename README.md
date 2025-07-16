# Bulker - Parallel Security Tools Runner

🚀 **High-performance parallel execution framework** for security tools with automatic optimizations, real-time output display, and intelligent file handling.

## ✨ Features

- **🔄 Parallel Processing**: Split input and run multiple tool instances simultaneously
- **⚡ Auto-Optimization**: Automatic performance tuning for each supported tool
- **📺 Real-time Output**: Live stdout display while maintaining clean result files
- **🎯 Smart File Handling**: Automatic output consolidation and backup management
- **🔧 Flexible Arguments**: Extra args support with `-e` flag
- **📊 Performance Metrics**: Detailed execution statistics and timing
- **🛡️ Signal Handling**: Graceful interruption with partial results preservation

## 🛠️ Supported Tools

| Tool | Description | Auto-Optimizations | Use Case |
|------|-------------|-------------------|----------|
| **httpx** | HTTP toolkit | Filters output flags | Web probing, status checking |
| **arjun** | Parameter discovery | `-t 10 -d 0 --rate-limit 50 -T 5` | Parameter fuzzing |
| **ffuf** | Web fuzzer | `-t 20 -p 0.1 -rate 100 -timeout 5` | Directory/file fuzzing |
| **echo** | Text processing | Direct processing | Data manipulation |

## 📦 Installation

```bash
# Clone and build
git clone <repository>
cd bulker
go build -o bulker.exe .

# Or download pre-built binary
```

## 🚀 Quick Start

```bash
# List available tools and their optimizations
.\bulker.exe list

# Run httpx with auto-optimization
.\bulker.exe run httpx -i domains.txt -w 4 -- -sc -rt -title

# Run arjun with custom method
.\bulker.exe run arjun -i urls.txt -w 2 -e '-m' -e 'POST' -- -oT params.txt

# Run ffuf with authorization header
.\bulker.exe run ffuf -i targets.txt -w 3 -e '-H' -e 'Auth: Bearer token' -- -w wordlist.txt -u https://target.com/FUZZ
```

## 📋 Usage

### Basic Syntax

```bash
.\bulker.exe run [tool] -i [input_file] [options] [-- tool_flags]
.\bulker.exe list                    # Show available tools
```

### Core Options

| Flag | Description | Default | Example |
|------|-------------|---------|---------|
| `-i, --input` | Input file path (required) | - | `-i urls.txt` |
| `-o, --output` | Output file path | `output.txt` | `-o results.txt` |
| `-w, --workers` | Number of parallel workers | `4` | `-w 8` |
| `-e, --extra-args` | Extra arguments for tool | `[]` | `-e '-H' -e 'Custom: header'` |

### Advanced Examples

#### HTTPx - Web Probing
```bash
# Basic probing with status codes and response time
.\bulker.exe run httpx -i domains.txt -w 4 -- -sc -rt -title

# With custom headers and technology detection
.\bulker.exe run httpx -i urls.txt -w 6 -e '-H' -e 'User-Agent: Scanner/1.0' -- -sc -rt -tech-detect

# Silent mode with JSON output
.\bulker.exe run httpx -i targets.txt -- -silent -json -sc -cl
```

#### Arjun - Parameter Discovery
```bash
# GET method parameter discovery
.\bulker.exe run arjun -i urls.txt -w 2 -- -oT parameters.txt

# POST method with custom wordlist
.\bulker.exe run arjun -i endpoints.txt -w 3 -e '-m' -e 'POST' -e '-w' -e 'custom.txt' -- -oT post_params.txt

# Passive discovery with multiple methods
.\bulker.exe run arjun -i targets.txt -e '--passive' -e '-m' -e 'GET,POST' -- -oT all_params.txt
```

#### Ffuf - Web Fuzzing
```bash
# Directory fuzzing
.\bulker.exe run ffuf -i baseUrls.txt -w 4 -- -w directories.txt -u https://target.com/FUZZ -mc 200,403

# File extension fuzzing with filters
.\bulker.exe run ffuf -i urls.txt -w 3 -- -w filenames.txt -e '.php,.asp,.jsp' -u https://target.com/FUZZ -fs 42

# Subdomain fuzzing with custom matcher
.\bulker.exe run ffuf -i domains.txt -e '-H' -e 'Host: FUZZ.target.com' -- -w subdomains.txt -u https://target.com/ -mc 200
```

## 🏗️ Architecture

### File Processing Flow
```
Input File → Chunking → Parallel Workers → Output Consolidation
     ↓            ↓           ↓                    ↓
  domains.txt  chunk_0.txt  httpx worker 0    output.txt
               chunk_1.txt  httpx worker 1        ↑
               chunk_2.txt  httpx worker 2        │
                    ...          ...         (consolidated)
```

### Config-Based Tool System
```
User Command → Tool Detection → Config Lookup → Command Building → Execution
     ↓              ↓              ↓               ↓              ↓
  "run arjun"  ConfigManager  config.json  Template Fill  "arjun -u {input} -t 10 -d 0"
```

### Configuration Structure
The tool configurations are defined in `config.json`:
```json
{
  "tools": {
    "httpx": {
      "description": "Fast HTTP toolkit for probing",
      "needsFileChunk": true,
      "command": "httpx -l {input} {args}",
      "autoOptimizations": [],
      "filterFlags": ["-o"]
    }
  }
}
```

## 📊 Performance Features

### Real-time Monitoring
- **Live Progress**: See tool output in real-time
- **Performance Metrics**: Execution time, memory usage, throughput
- **Task Status**: Completed, running, failed counts
- **Error Handling**: Stderr capture and display

### Example Performance Output
```
[INFO] Total lines: 1000, Workers: 4, Chunk size: 250
[TASK-0] Started: worker_0 (PID: 12345)
[TASK-1] Started: worker_1 (PID: 12346)
...
[PERF] Total execution time: 45.2s
[PERF] Average task time: 11.3s
[PERF] Tasks completed: 4, Tasks failed: 0
```

## 🔧 Configuration

### Tool-Specific Settings

**HTTPx Auto-Optimizations:**
- Automatically filters `-o` flags (uses bulker output instead)
- Preserves all other flags and user preferences

**Arjun Auto-Optimizations:**
- `-t 10`: Optimized thread count
- `-d 0`: No delay between requests  
- `--rate-limit 50`: Controlled request rate
- `-T 5`: Reduced timeout for faster scanning

**Ffuf Auto-Optimizations:**
- `-t 20`: Balanced thread count
- `-p 0.1`: Minimal delay for stability
- `-rate 100`: High request throughput
- `-timeout 5`: Quick timeout for efficiency

## 📁 Output Management

### File Structure
```
project/
├── input.txt                    # Original input
├── output.txt                   # Consolidated results
├── output_20250716_120000.txt   # Auto-backup of existing output
└── [temp files cleaned automatically]
```

### Output Features
- **Automatic Backup**: Existing output files are timestamped and preserved
- **Clean Results**: Only final results in output file (no progress messages)
- **Thread-Safe Writing**: Concurrent workers write safely to shared output
- **Format Preservation**: Tool-specific output formats maintained (JSON, text, etc.)

## 🚦 Error Handling

### Graceful Interruption
```bash
# Press Ctrl+C during execution
[WARN] Received interrupt signal, cleaning up...
[INFO] Partial results saved to: output.txt
```

### Common Issues & Solutions
| Issue | Solution |
|-------|----------|
| Tool not found | Ensure tool is in PATH or provide full path |
| Permission denied | Check file permissions and antivirus settings |
| High memory usage | Reduce worker count with `-w` flag |
| Network timeouts | Tools auto-optimize timeouts, or use `-e` for custom values |

## 🔍 Troubleshooting

### Debug Mode
```bash
# View detailed execution
.\bulker.exe run httpx -i urls.txt -w 1 -- -verbose

# Check tool installation
httpx -version
arjun -h
ffuf -V
```

### Performance Tuning
```bash
# CPU-bound tasks (parsing, filtering)
.\bulker.exe run tool -i input.txt -w [CPU_CORES]

# Network-bound tasks (web requests)  
.\bulker.exe run tool -i input.txt -w [2-4x CPU_CORES]

# Memory-limited environments
.\bulker.exe run tool -i input.txt -w 2
```

## 🤝 Contributing

1. Fork the repository
2. Create feature branch: `git checkout -b feature/amazing-feature`
3. Add new tool configuration to `config.json`
4. Test thoroughly and submit PR

### Adding New Tools
Simply add your tool configuration to `config.json`:

```json
{
  "tools": {
    "mytool": {
      "description": "My awesome security tool",
      "needsFileChunk": false,
      "handlesFileOutput": true,
      "command": "mytool -u {input} {autoOptimizations} {args}",
      "autoOptimizations": ["-t", "10", "--fast"],
      "filterFlags": ["-o"],
      "examples": [
        "bulker run mytool -i targets.txt -w 4 -- --scan-all"
      ]
    }
  }
}
```

### Configuration Options
- **needsFileChunk**: Tool needs input as file (like httpx -l file.txt)
- **handlesFileOutput**: Tool manages its own output file (like arjun -oT output.txt)
- **command**: Command template with placeholders:
  - `{input}`: Input data (filename or single line)
  - `{autoOptimizations}`: Auto-optimization flags
  - `{args}`: User-provided arguments
- **autoOptimizations**: Default performance flags
- **filterFlags**: Flags to remove from user args (bulker handles them)

## 📜 License

MIT License - see LICENSE file for details

## 🙏 Acknowledgments

- **HTTPx**: [projectdiscovery/httpx](https://github.com/projectdiscovery/httpx)
- **Arjun**: [s0md3v/Arjun](https://github.com/s0md3v/Arjun)  
- **Ffuf**: [ffuf/ffuf](https://github.com/ffuf/ffuf)
- **Cobra**: [spf13/cobra](https://github.com/spf13/cobra)

---

**⚡ Bulker - Making security tools faster, together! ⚡** 