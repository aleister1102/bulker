# Bulker - Parallel Processing Tool

Tool for running command line tools in parallel through tmux detach with input file splitting and interrupt handling capabilities.

## Features

- Split input file into chunks for parallel processing
- Run commands through tmux detach (background)
- Support multi-threading with worker limits
- Interrupt handling to collect partial results
- Thread-safe output writing to single file
- Cleanup mode to check existing output from interrupted runs

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
- `--cleanup`: Cleanup mode - check existing output from interrupted run

### Placeholders in Command

- `{input}`: Replaced with chunk file path
- `{output}`: No longer used (removed for single file output)

### Examples

#### Run grep in parallel

```bash
./bulker run grep --input data.txt --workers 8 --chunk-size 500 -- -i "pattern" {input}
```

#### Run custom processing script

```bash
./bulker run python --input big_data.txt --workers 4 -- process.py {input}
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
5. All workers write output to single shared file with thread safety
6. If interrupted (Ctrl+C), cleanup and keep partial results

## Output Structure

```
output/
├── chunk_0000.txt    # Input chunks
├── chunk_0001.txt
└── output.txt        # Single output file with all results
```

## Thread Safety

- All workers write to a single shared output file
- Mutex synchronization prevents race conditions
- Output order may vary due to parallel processing
- File writes are immediately synced to disk

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
- When receiving interrupt signal, tool will cleanup and save partial results to output file

## Platform Support

- **Unix/Linux/macOS**: Uses tmux for parallel processing
- **Windows**: Uses background processes instead of tmux 