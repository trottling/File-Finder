# FileFinder

Мощная кроссплатформенная CLI-утилита для поиска по файлам и архивам с поддержкой сложных паттернов, whitelist/blacklist
расширений, поиска в глубину, fail-fast и потоковой обработки.

---

## 🚀 Быстрый старт

1. **Собери**
    ```bash
    go build -o filefinder ./cmd/finder.go
    ```

2. **Создай файл паттернов**  
   Пример `patterns.txt`:
    ```
    re:\bsecret\b
    plain:password
    plain:i:Token
    ```

3. **Запусти**
    ```bash
    ./filefinder --pattern-file patterns.txt --whitelist txt,log --archives --threads 8 /var/log
    ```

---

## 🔥 Ключевые фичи

- Поиск по всем дискам и внешним носителям (автоматически определяет root для Windows, Linux, MacOS)
- Поддержка архивов: `zip`, `tar`, `gz`, `bz2`, `xz`, `rar`
- Гибкая фильтрация: whitelist и blacklist расширений
- Глубина поиска (`--depth N`)
- Fail-fast: остановка на первой ошибке (`--fail-fast`)
- Многопоточность (выбирай — хоть 100+ потоков!)
- Лимит на количество файлов в архиве (анти zip-бомба)
- Красивый лог, отчёт по итогам сканирования

---

## 🛠️ Аргументы командной строки

| Флаг                    | Описание                                                                                    | Пример                              |
|-------------------------|---------------------------------------------------------------------------------------------|-------------------------------------|
| `--pattern-file`        | Путь к файлу с паттернами (обязателен)                                                      | `--pattern-file patterns.txt`       |
| `--whitelist`           | Список разрешённых расширений (через запятую)                                               | `--whitelist txt,log`               |
| `--blacklist`           | Список игнорируемых расширений                                                              | `--blacklist jpg,png`               |
| `--logfile`             | Путь к файлу лога (по умолчанию stdout)                                                     | `--logfile finder.log`              |
| `--threads`             | Кол-во потоков (воркеров)                                                                   | `--threads 8`                       |
| `--save-full`           | Сохранять полностью файл при совпадении (а не только строки)                                | `--save-full`                       |
| `--save-full-folder`    | Папка для сохранённых файлов, по умолчанию `/found_files`                                   | `--save-full-folder ./../result`    |
| `--save-matches-file`   | Файл для сохранения всех найденных строк в один файл                                        | `--save-matches-file result.txt`    |  
| `--save-matches-folder` | Папка для сохранения найденных строк в файлы с именем паттерна по которому они были найдены | `--save-matches-folder ./../result` |  
| `--archives`            | Искать и в архивах                                                                          | `--archives`                        |
| `--depth`               | Глубина поиска (0 — безлимит)                                                               | `--depth 3`                         |
| `--timeout`             | Ограничить время поиска (пример: 10m, 1h)                                                   | `--timeout 10m`                     |
| `--fail-fast`           | Остановиться на первой ошибке                                                               | `--fail-fast`                       |

**Пример:**

```bash
./filefinder --pattern-file patterns.txt --whitelist txt,log --archives --threads 8 --depth 2 /home /mnt/flash
```

---

## 🎯 Формат паттернов

Файл паттернов — это обычный текстовый файл, где каждая строка это паттерн поиска:

```
re: — регулярка, Go-style (re:пароль\d+)
plain: — просто строка (чувствительно к регистру)
plain:i: — просто строка, не чувствительно к регистру
```

---

## 📝 Примеры запуска

Простой поиск по всем дискам:

```bash
./filefinder --pattern-file patterns.txt
```

С фильтром по расширениям и глубиной:

```bash
./filefinder --pattern-file patterns.txt --whitelist txt,md,log --depth 2 /home/user/Documents
```

Поиск в архивах и остановка на ошибке:

```bash
./filefinder --pattern-file patterns.txt --archives --fail-fast /var/data
```

---

## 💡 FAQ

* Что делать, если ищет слишком долго?
  Ограничь глубину через `--depth N` или таймаутом `--timeout 5m`.

* Почему не запускается?
  Не забудь флаг `--pattern-file`.

* Можно ли искать по конкретным папкам?
  Да, просто передай их в конце команды:

```bash
 ./filefinder ... /home /mnt/usb
```

* Где лог?
  По умолчанию в `stdout`, или задай флагом `--logfile`.

---

## 🧪 Тестирование

Протестировать всё можно так:

```bash
go test ./internal/scanner/...
```

---

## ⚠️ Анти zip-bomb

В архиве обрабатывается не больше 10 000 файлов — иначе скипается (инфа в логе).

---

## 🚀 Быстрый старт с Docker Compose

1. **Положи свои файлы и `patterns.txt` в папку `data/`.**
2. (Опционально) Создай папку `found_files/` для сохранения совпадений с `--save-full-folder`.
3. Запусти:

    ```bash
    docker-compose up --build
    ```

4. Логи будут прямо в консоли, результаты поиска — в том же `found_files/` (если включишь).

**Изменить параметры запуска (паттерны, расширения, глубина) можно прямо в `docker-compose.yml`, секция `command:`.**

---

## ℹ️ Пример структуры для запуска:

```
FileFinder/
├── data/
│ ├── patterns.txt
│ └── ... (любые файлы и папки для сканирования)
├── found_files/ # если используете --save-full-folder
├── Dockerfile
├── docker-compose.yml
└── ... (исходный код)
```

---

### ⚡️ Пример запуска с кастомным набором флагов:

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