# Bulker - Parallel Processing Tool

Tool for running command line tools in parallel through tmux detach with input file splitting and interrupt handling capabilities.

## Features

- Split input file into chunks for parallel processing (auto-calculated based on workers)
- Run commands through tmux detach (background)
- Support multi-threading with worker limits
- Interrupt handling to collect partial results
- Thread-safe output writing to single file
- Auto-generated tmux session names based on command
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
- `--cleanup`: Cleanup mode - check existing output from interrupted run

### Placeholders in Command

- `{input}`: Replaced with chunk file path
- `{output}`: No longer used (removed for single file output)

### Examples

#### Run grep in parallel

```bash
./bulker run grep --input data.txt --workers 8 -- -i "pattern" {input}
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

1. Tool counts total lines in input file
2. Calculates chunk size automatically (total_lines / workers)
3. Splits input file into chunks
4. Creates tmux session with auto-generated name based on command
5. Runs command on each chunk in separate tmux windows/processes
6. Monitors progress and reports status
7. All workers write output to single shared file with thread safety
8. If interrupted (Ctrl+C), cleanup and keep partial results

## Output Structure

```
output/
├── chunk_0000.txt    # Input chunks
├── chunk_0001.txt
└── output.txt        # Single output file with all results
```

## Auto Session Naming

- Session names are automatically generated from command name
- Invalid characters are replaced with underscores
- If session already exists, a number suffix is added (e.g., `grep_1`, `grep_2`)
- Examples:
  - `grep` → `grep`
  - `python script.py` → `python`
  - `./my-tool` → `my-tool`
  - `/usr/bin/find` → `find`

## Thread Safety

- All workers write to a single shared output file
- Mutex synchronization prevents race conditions
- Output order may vary due to parallel processing
- File writes are immediately synced to disk

## Tmux Management

Tool creates a tmux session with auto-generated name and creates separate windows for each worker. You can:

```bash
# View sessions
tmux list-sessions

# Attach to session for monitoring (session name will be shown in output)
tmux attach-session -t [session_name]

# View windows
tmux list-windows -t [session_name]
```

## Error Handling

- If tmux is not available, tool will report error (Unix) or use background processes (Windows)
- If input file does not exist, tool will report error
- If command fails on a chunk, tool will report warning and continue
- When receiving interrupt signal, tool will cleanup and save partial results to output file

## Platform Support

- **Unix/Linux/macOS**: Uses tmux for parallel processing
- **Windows**: Uses background processes instead of tmux 