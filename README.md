# FileFinder

FileFinder is a fast, concurrent file content searcher for large directories and archives.

## Features
- Recursive search in directories and archives (zip, tar, gz, bz2, xz, rar)
- Pattern file supports plain text and regular expressions
- Whitelist/blacklist by file extension
- Progress bar and error reporting
- Optionally saves full file content for matches
- Multithreaded (configurable)

## Usage

```sh
go run ./cmd/finder.go --pattern-file patterns.txt --whitelist .go,.txt /path/to/scan
```

### CLI Options
- `--pattern-file` (required): Path to file with patterns (see below)
- `--whitelist`: Comma-separated list of file extensions to include (e.g. `.go,.txt`)
- `--blacklist`: Comma-separated list of file extensions to exclude
- `--threads`: Number of concurrent workers (default: 100)
- `--archives`: Scan inside supported archive files
- `--save-full`: Save the entire file content for each match
- `--timeout`: Timeout for the scan (e.g. `10m`, `1h`)

If no search path is provided, all available drives (Windows) or `/` (Unix) will be scanned.

## Pattern File Format

Each line in the pattern file is a separate pattern. Two types are supported:

- **Plain text**: Just write the string to search for. Example:
  ```
  password
  secret
  TODO
  ```
- **Regular expression**: Start the line with `re:`. Example:
  ```
  re:\d{4}-\d{2}-\d{2}
  re:password\s*=
  ```

**Note:** Plain text patterns are case-sensitive and match substrings. Regex patterns use Go's regexp syntax.

## Example `patterns.txt`
```
password
re:\d{4}-\d{2}-\d{2}
```

## Output

Results are printed to the log (stdout or file if configured). Each match includes file path, line number, and optionally the full file content if `--save-full` is set.

## Example
```sh
go run ./cmd/finder.go --pattern-file patterns.txt --whitelist .go,.txt ./myproject
```

## Requirements
- Go 1.20+
