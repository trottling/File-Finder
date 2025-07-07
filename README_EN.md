## 🇷🇺  [Русская версия](./README.md)

# FileFinder

A powerful cross-platform CLI utility for searching files and archives with support for complex patterns, whitelist/blacklist
extensions, depth-first search, fail-fast and stream processing.

---

## 🚀 Quick Start

1. **Build**
```bash
go build -o filefinder ./cmd/finder.go
```

2. **Create a patterns file**

Example `patterns.txt`:
```
re:\bsecret\b
plain:password
plain:i:Token
```

3. **Run**
```bash
./filefinder --pattern-file patterns.txt --whitelist txt,log --archives --threads 8 /var/log
```

---

## 🔥 Key Features

- Search all disks and external media (automatically detects root for Windows, Linux, MacOS)
- Support for archives: `zip`, `tar`, `gz`, `bz2`, `xz`, `rar`
- Flexible filtering: whitelist and blacklist of extensions
- Search depth (`--depth N`)
- Fail-fast: stop on the first error (`--fail-fast`)
- Multithreading (choose - at least 100+ threads!)
- Limit on the number of files in the archive (anti-zip bomb)
- Beautiful log, report on the results of the scan

---

## 🛠️ Command line arguments

| Flag | Description | Example |
|------------------------|---------------------------------------------------------------------------------------------|-------------------------------------|
| `--pattern-file` | Path to the file with patterns (required) | `--pattern-file patterns.txt` |
| `--whitelist` | List of allowed extensions (separated by commas) | `--whitelist txt,log` |
| `--blacklist` | List of ignored extensions | `--blacklist jpg,png` |
| `--logfile` | Path to log file (default stdout) | `--logfile finder.log` |
| `--threads` | Number of threads (workers) | `--threads 8` |
| `--save-full` | Save the entire file on match (not just the lines) | `--save-full` |
| `--save-full-folder` | Folder for saved files, default `/found_files` | `--save-full-folder ./../result` |
| `--save-matches-file` | File for saving all found lines to one file | `--save-matches-file result.txt` |
| `--save-matches-folder` | Folder for saving found strings in files with the name of the pattern by which they were found | `--save-matches-folder ./../result` |
| `--archives` | Search in archives too | `--archives` |
| `--depth` | Search depth (0 — unlimited) | `--depth 3` |
| `--timeout` | Limit search time (example: 10m, 1h) | `--timeout 10m` |
| `--fail-fast` | Stop on first error | `--fail-fast` |

**Example:**

```bash
./filefinder --pattern-file patterns.txt --whitelist txt,log --archives --threads 8 --depth 2 /home /mnt/flash
```

---

## 🎯 Pattern format

A pattern file is a regular text file, where each line is a search pattern:

```
re: — regular expression, Go-style (re:password\d+)
plain: — just a string (case-sensitive)
plain:i: — just a string, case-insensitive
```

---

## 📝 Launch examples

Simple search across all disks:

```bash
./filefinder --pattern-file patterns.txt
```

With a filter by extensions and depth:

```bash
./filefinder --pattern-file patterns.txt --whitelist txt,md,log --depth 2 /home/user/Documents
```

Search archives and stop on error:

```bash
./filefinder --pattern-file patterns.txt --archives --fail-fast /var/data
```

---

## 💡 FAQ

* What to do if it searches too long?
  Limit the depth with `--depth N` or timeout `--timeout 5m`.

* Why doesn't it start?
  Don't forget the `--pattern-file` flag.

* Is it possible to search specific folders?
  Yes, just pass them at the end of the command:

```bash
./filefinder ... /home /mnt/usb
```

* Where is the log?
  By default in `stdout`, or set with the `--logfile` flag.

---

## 🧪 Testing

You can test everything like this:

```bash
go test ./internal/scanner/...
```

---

## ⚠️ Anti zip-bomb

No more than 10,000 files are processed in the archive - otherwise they are skipped (info in the log).

---

## 🚀 Quick Start with Docker Compose

1. **Put your files and `patterns.txt` in `data/` folder.**
2. (Optional) Create `found_files/` folder to save matches with `--save-full-folder`.

3. Run:

```bash
docker-compose up --build
```

4. Logs will be right in the console, search results - in the same `found_files/` (if you enable it).

**You can change the launch parameters (patterns, extensions, depth) directly in `docker-compose.yml`, section `command:`.**

---

## ℹ️ Example of the structure for launch:

```
FileFinder/
├── data/
│ ├── patterns.txt
│ └── ... (any files and folders to scan)
├── found_files/ # if you use --save-full-folder
├── Dockerfile
├── docker-compose.yml
└── ... (source code)
```

---

### ⚡️ Example of launching with a custom set flags:

```yaml
command: > 
  --pattern-file /data/patterns.txt 
  --whitelist txt,log 
  --archives 
  --threads 16 
  --depth 3 
  --save-full 
  --save-full-folder /found_files 
  /data
```