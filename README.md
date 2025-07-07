# Bulker - Parallel Processing Tool

Tool for running command line tools in parallel through tmux detach with input file splitting and interrupt handling capabilities.

## Features

- Split input file into chunks for parallel processing
- Run commands through tmux detach (background)
- Support multi-threading with worker limits
- Interrupt handling to collect partial results
- Merge results from chunks
- Cleanup mode to collect results from interrupted runs

## Installation

```bash
go mod tidy
go build -o bulker
```

## Usage

### Basic Syntax

```bash
./bulker run [command] --input [input_file] [options]
```

### Parameters

- `--input, -i`: Input file path (required)
- `--output, -o`: Output directory (default: "output")
- `--workers, -w`: Number of parallel workers (default: 4)
- `--chunk-size, -c`: Chunk size (lines) (default: 1000)
- `--session, -s`: Tmux session name (default: "bulker")
- `--cleanup`: Cleanup mode - collect results from interrupted run

### Placeholders in Command

- `{input}`: Replaced with chunk file path
- `{output}`: Replaced with result file path

### Examples

#### Run grep in parallel

```bash
./bulker run grep --input data.txt --workers 8 --chunk-size 500 -- -i "pattern" {input} > {output}
```

#### Run custom processing script

```bash
./bulker run python --input big_data.txt --workers 4 -- process.py {input} {output}
```

#### Cleanup after interrupt

```bash
./bulker run --cleanup --output output_dir
```

## Workflow

1. Tool splits input file into chunks
2. Creates new tmux session (or uses background processes on Windows)
3. Runs command on each chunk in separate tmux windows/processes
4. Monitors progress and reports status
5. When completed, merges all result files
6. If interrupted (Ctrl+C), cleanup and merge partial results

## Output Structure

```
output/
├── chunk_0000.txt    # Input chunks
├── chunk_0001.txt
├── result_0000.txt   # Result files
├── result_0001.txt
└── merged_result.txt # Final merged result
```

## Tmux Management

Tool creates a tmux session with specified name (default: "bulker") and creates separate windows for each worker. You can:

```bash
# View sessions
tmux list-sessions

# Attach to session for monitoring
tmux attach-session -t bulker

# View windows
tmux list-windows -t bulker
```

## Error Handling

- If tmux is not available, tool will report error (Unix) or use background processes (Windows)
- If input file does not exist, tool will report error
- If command fails on a chunk, tool will report warning and continue
- When receiving interrupt signal, tool will cleanup and merge available results

## Platform Support

- **Unix/Linux/macOS**: Uses tmux for parallel processing
- **Windows**: Uses background processes instead of tmux 