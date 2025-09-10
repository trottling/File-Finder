## 🇬🇧  [English version](README_EN.md)

# FileFinder

Мощная кроссплатформенная CLI-утилита для поиска по файлам и архивам. Поддерживает plain/regex паттерны,
whitelist/blacklist расширений, ограничение глубины, fail-fast, потоковую обработку и сохранение полных файлов при
совпадении.

---

## 🚀 Быстрый старт

1. Сборка

```bash
go build -o filefinder ./cmd/main.go
```

2. Паттерны - создайте `patterns.txt`

```
re:\bsecret\b
plain:password
plain:i:Token
```

3. Запуск

```bash
./filefinder \
  --pattern-file patterns.txt \
  --whitelist txt,log \
  --archives \
  --threads 8 \
  /var/log
```

---

## 🔥 Что внутри

* Поиск по всем дискам и подключенным томам - автодетект root для Windows/Linux/macOS
* Архивы: zip, tar, gz, bz2, xz, rar, 7z, zst и др.
* Фильтрация расширений: whitelist и blacklist
* Глубина обхода `--depth N` - режет дерево рано, экономит время
* Fail-fast `--fail-fast` - остановка на первой ошибке
* Потоковая обработка - десятки-сотни воркеров
* Анти zip-бомба - ограничение количества файлов в архиве
* Понятные логи и финальная статистика
* Сохранение полных файлов при первом совпадении без загрузки в память - через temp+rename

---

## 🛠️ Аргументы

| Флаг                    | Описание                                                   | Пример                               |
|-------------------------|------------------------------------------------------------|--------------------------------------|
| `--pattern-file`        | Путь к файлу с паттернами - обязателен                     | `--pattern-file patterns.txt`        |
| `--whitelist`           | Только эти расширения - без точки, через запятую           | `--whitelist txt,log,json`           |
| `--blacklist`           | Исключить эти расширения                                   | `--blacklist jpg,png`                |
| `--archives`            | Включить скан архивов                                      | `--archives`                         |
| `--depth`               | Максимальная глубина (0 - безлимит)                        | `--depth 3`                          |
| `--threads`             | Кол-во воркеров. 0 - авто (примерно 4x от CPU, минимум 32) | `--threads 200`                      |
| `--timeout`             | Глобальный таймаут скана                                   | `--timeout 10m`                      |
| `--fail-fast`           | Остановить процесс при первой ошибке                       | `--fail-fast`                        |
| `--save-full`           | Сохранять целиком файл при первом совпадении               | `--save-full`                        |
| `--save-full-folder`    | Папка для сохранённых файлов при `--save-full`             | `--save-full-folder ./found_files`   |
| `--save-matches-file`   | Сохранить все найденные строки в один файл                 | `--save-matches-file all.txt`        |
| `--save-matches-folder` | Сохранить найденные строки по файлам на каждый паттерн     | `--save-matches-folder ./by_pattern` |
| `--logfile`             | Писать логи в файл                                         | `--logfile finder.log`               |
| `--log-level`           | Уровень логов: debug, info, warn, error                    | `--log-level debug`                  |

* Если задан `--whitelist`, то `--blacklist` игнорируется - whitelist главнее.
* Путь(и) для скана передаются последними аргументами. Если не передать - авто-детект всех корней ОС.

---

## 🎯 Формат паттернов

Каждая строка в `patterns.txt` - отдельный паттерн:

```
# комментарии допускаются
re:^id=\d{3}$      # Go-regex, префикс re:
plain:foo          # plain-строка, чувствительная к регистру (префикс plain: не обязателен)
plain:i:Token      # plain-строка, регистр игнорируется
```

Коротко:

* `re:` - компилируется как regexp в Go
* `plain:` - подстрока, чувствительная к регистру
* `plain:i:` - подстрока без учёта регистра

---

## 📝 Примеры

Поиск по всем дискам с автодетектом:

```bash
./filefinder --pattern-file patterns.txt
```

Ограничить расширениями и глубиной:

```bash
./filefinder --pattern-file patterns.txt --whitelist txt,md,log --depth 2 /home/user/Documents
```

Архивы + fail-fast:

```bash
./filefinder --pattern-file patterns.txt --archives --fail-fast /var/data
```

Сохранить все найденные строки в один файл:

```bash
./filefinder --pattern-file patterns.txt --save-matches-file ./matches.txt /var/log
```

Сохранять полные файлы с совпадениями:

```bash
./filefinder --pattern-file patterns.txt --save-full --save-full-folder ./found /etc /var/log
```

---

## 🧪 Тесты

```bash
go test -v ./...
```

---

## ⚙️ Makefile

Минимальный удобный сценарий:

```bash
make build     # собрать ./bin/filefinder
make run       # собрать и запустить с дефолтными флагами
make test      # прогнать тесты
make clean     # очистить bin
```

`Makefile` уже настроен на `./cmd/main.go` и использует `patterns.txt` по умолчанию.

---

## 🐳 Docker

Dockerfile на базе distroless\:nonroot. Пример сборки и запуска:

```bash
# build
docker build -t filefinder:latest .

# run - сканируем ./data, паттерны из ./patterns.txt, найденные полные файлы кладём в ./found_files
docker run --rm \
  -v "$PWD/data:/scan" \
  -v "$PWD/patterns.txt:/patterns.txt" \
  -v "$PWD/found_files:/found_files" \
  filefinder:latest \
  --pattern-file /patterns.txt \
  --threads 100 \
  --archives \
  /scan
```

* В distro-less нет шелла - папка `/found_files` создаётся через `WORKDIR` в Dockerfile.
* Контейнер запускается под nonroot.

---

## ⚠️ Анти zip-бомба

Внутри архива обрабатывается максимум 10000 файлов - при превышении архив скипается. Это защищает от zip-бомб.

---

## Структура проекта

```
cmd/
  main.go
internal/
  logger.go
  options.go
  matcher.go
  fs.go
  reader.go
  scanner.go
```
