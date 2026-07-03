# Спецификация доработок backend-микросервисов Flashbacks

> Дата: 2026-07-02
> Автор: GigaCode Code Review
> Версия: 1.0

## Содержание

1. [Общие требования](#1-общие-требования)
2. [Приоритет 1: Безопасность и стабильность](#2-приоритет-1-безопасность-и-стабильность)
   - [2.1. Разбить handlers.go на отдельные файлы](#21-разбить-handlersgo-на-отдельные-файлы)
   - [2.2. Внедрить DI-контейнер](#22-внедрить-di-контейнер)
   - [2.3. Прокидывать context.Context через все слои](#23-прокидывать-contextcontext-через-все-слои)
   - [2.4. Перейти с log.Printf на slog](#24-перейти-с-logprintf-на-slog)
3. [Приоритет 2: Производительность](#3-приоритет-2-производительность)
   - [3.1. Закэшировать GalleryAccess](#31-закэшировать-galleryaccess)
   - [3.2. Вынести генерацию тумбнейлов из пагинации в фон](#32-вынести-генерацию-тумбнейлов-из-пагинации-в-фон)
   - [3.3. Пул экземпляров exiftool](#34-пул-экземпляров-exiftool)
4. [Приоритет 3: Архитектура и чистота кода](#4-приоритет-3-архитектура-и-чистота-кода)
   - [4.1. Убрать прямой доступ к s.db из хендлеров](#41-убрать-прямой-доступ-к-sdb-из-хендлеров)
   - [4.2. Объединить реализации thumbnail-кэша](#42-объединить-реализации-thumbnail-кэша)
   - [4.3. Унифицировать паттерн ретраев](#43-унифицировать-паттерн-ретраев)
   - [4.4. Унифицировать OCR/EXIF healthcheck](#44-унифицировать-оcrexif-healthcheck)
5. [Приоритет 4: Developer Experience и CI](#5-приоритет-4-developer-experience-и-ci)
   - [5.1. Добавить make test-all](#51-добавить-make-test-all)
   - [5.2. Добавить golangci-lint](#52-добавить-golangci-lint)
   - [5.3. Добавить make docker-build-all](#53-добавить-make-docker-build-all)
6. [Приложение A: Matrix зависимостей](#приложение-a-matrix-зависимостей)

---

## 1. Общие требования

- Все изменения должны сохранять совместимость API.
- Тесты должны проходить после каждого изменения.
- Изменения должны быть атомарными — один PR на один пункт спецификации (если не указано иное).
- Все Go-файлы следуют существующему стилю кодирования (PascalCase экспорт, camelCase неэкспорт, explicit error handling).
- Новый код должен использовать `slog`.

---

## 2. Приоритет 1: Безопасность и стабильность

### 2.1. Разбить handlers.go на отдельные файлы

**Файл (текущий):** `backend/api-service/internal/interfaces/handler/handlers.go` (~3500 строк)

**Проблема:** Один файл содержит все бизнес-хендлеры — дубликаты, сканирование, галерея, настройки, корзина, users, OCR, LLM, синхронизация, тумбнейлы, гео. Нарушение SRP: сложно навигировать, ревьюить, тестировать.

**Требование:** Разбить на тематические файлы:

| Новый файл | Содержимое | Ориентировочный объём |
|---|---|---|
| `handlers_duplicates.go` | `handleGetDuplicates`, `handleDeleteFiles`, `handleBatchDelete` | ~300 строк |
| `handlers_scan.go` | `handleScan`, `handleFastScan`, `handleGetStatus`, `handleGetSyncStatus` | ~400 строк |
| `handlers_gallery.go` | `handleGetGalleryImages`, `handleGetGalleryCalendar`, `handleGetCalendarMonthInfo`, `handleGetCalendarAllDates`, `handleGetCalendarSeek`, `handleGetGalleryClusters`, `handleGetGeoImages`, `handleServeImage`, `handleServeOcrImage` | ~600 строк |
| `handlers_folders.go` | `handleGetFolders`, `handleAddFolder`, `handleRemoveFolder`, `handleGetFolderPatterns` | ~200 строк |
| `handlers_trash.go` | `handleGetTrashInfo`, `handleCleanTrash`, `handleListTrashFiles`, `handleRestoreTrashFile`, `handleDeleteTrashFile` | ~300 строк |
| `handlers_ocr.go` | `handleGetOCRStatus`, `handleStartOcrClassification`, `handleStartOcrClassificationIncremental`, `handleStopOcrClassification`, `handleGetOcrClassificationStatus`, `handleGetOcrDocuments`, `handleGetOcrData` | ~400 строк |
| `handlers_llm.go` | `handleGetLlmSettings`, `handleUpdateLlmSettings`, `handleCreateLlmProvider`, `handleUpdateLlmProvider`, `handleDeleteLlmProvider`, `handleLlmRecognize`, `handleLlmRecognizeStatus`, `handleGetLlmRecognition`, `handleGetLlmModels`, `handleProbeEmbeddingDimension`, `handleGetImageTags` | ~500 строк |
| `handlers_thumbnail.go` | `handleThumbnail`, `handleThumbnailCacheStats`, `handleThumbnailCacheInvalidate`, `handleThumbnailCacheInvalidateAll`, `handleThumbnailCacheWarmup`, `handleThumbnailCacheEnable`, `handleThumbnailCacheDisable` | ~200 строк |
| `handlers_settings.go` | `handleGetSettings`, `handleUpdateSettings`, `handleGetUserSettings`, `handleUpdateUserSettings` | ~200 строк |
| `handlers_geo.go` | `handleGetImageMetadata`, `handleGetImagesMissingExif`, `handleGeocodeSearch`, `handleUpdateGps`, `handleBatchUpdateGps`, `handleGetLocationCandidates` | ~200 строк |
| `handlers_tag_scan.go` | `handleTagScanStatus`, `handleTagScanPause`, `handleTagScanResume` | ~100 строк |
| `handlers_embedding.go` | `handleEmbeddingStatus`, `handleEmbeddingStart`, `handleEmbeddingStop` | ~100 строк |

**Критерий приёмки:** `handlers.go` не существует или содержит менее 100 строк (только общие утилиты).

---

### 2.2. Внедрить DI-контейнер

**Файл (текущий):** `backend/api-service/cmd/server/main.go`

**Проблема:** 30+ явных аргументов в `NewServer()`. Ручной wiring всех зависимостей с дублированием конструкций типа:
```go
exifSvcClient := exifclient.NewHTTPExifClient(cfg.ExifServiceURL)
...
server := handler.NewServer(db, scanManager, ocrManager, llmOcrService, backgroundSync, tagScanManager, embeddingBackfill, thumbnailService, cfg, geolocationService, nominatimClient, mcpSrv, ag, agCfg, convService, exifSvcClient)
```

**Требование:** Внедрить google/wire (или uber/fx):

- **Вариант A (Wire — статическая генерация):**
  - Создать `wire.go` с `InitializeServer()`.
  - Создать `wire_gen.go` (генерируется Wire).
  - Разбить провайдеры на группы: `CoreSet`, `AppSet`, `HandlerSet`.
  
- **Вариант B (fx — runtime DI):**
  - Создать модули для каждой группы зависимостей.
  - `fx.Provide()` для каждого конструктора.
  - `fx.Invoke()` для старта фоновых задач.

**Критерий приёмки:** `main.go` содержит не более 15 строк значимой логики (без комментариев и разделителей). `NewServer` принимает не более 5 параметров.

---

### 2.3. Прокидывать context.Context через все слои

**Проблема:** Gin не прокидывает `context.Context` в слой сервисов. `background.Manager` поддерживает `context.Context`, но не везде используется.

**Требование:**

1. Создать утилиту для извлечения контекста из Gin:
```go
// helpers/context.go
func RequestContext(c *gin.Context) context.Context {
    // возвращает c.Request.Context()
}
```

2. Изменить сигнатуры всех service-методов, добавив `ctx context.Context` первым аргументом:
```go
// Было
func (s *AuthService) Login(login, password, ipAddress, userAgent string) (*LoginResult, error)

// Стало
func (s *AuthService) Login(ctx context.Context, login, password, ipAddress, userAgent string) (*LoginResult, error)
```

3. Использовать контекст для:
   - Cancellation: если клиент разорвал соединение, фоновые задачи должны прерываться.
   - Deadlines: таймауты на запросы к внешним сервисам (exif, ocr).
   - Propagation: передача идентификатора запроса (trace ID).

**Затрагиваемые сервисы (по цепочке):**

| Слой | Файлы |
|---|---|
| Handler | `handlers.go`, `auth_handlers.go` — получают `c.Request.Context()` |
| Application | `auth/service.go`, `imaging/scanner.go`, `imaging/ocr.go`, `geo/clustering.go` |
| Infrastructure | `exifclient/client.go`, `ocr/client.go`, `llm/*_client.go` |

**Критерий приёмки:** Ни один service-метод, вызываемый из handler, не принимает `context.Context`. Проходят unit-тесты.

---

### 2.4. Перейти с log.Printf на slog

**Проблема:** Во всех трёх микросервисах используется `log.Printf`/`log.Fatalf` — неструктурированное логирование.

**Требование:**

1. В каждом сервисе создать/использовать глобальный `*slog.Logger`:
```go
// internal/infrastructure/logger/logger.go
var Logger = slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
    Level: slog.LevelInfo,
}))
```

2. Заменить все `log.Printf(...)` на `slog.InfoContext(ctx, "...", "key", value)`.

3. Заменить `log.Fatalf(...)` на структурированный эквивалент.

4. Добавить атрибуты: `service`, `request_id`, `component`.

**Пример замены:**
```go
// Было
log.Printf("EXIF %s: GPS via float: lat=%.8f, lng=%.8f", baseName, lat, lng)

// Стало
slog.Info("extracted GPS coordinates",
    "service", "exif",
    "file", baseName,
    "method", "float",
    "lat", lat,
    "lng", lng,
)
```

**Матрица замен:**

| Сервис | Файлы | Прибл. кол-во замен |
|---|---|---|
| api-service | `scanner.go`, `ocr.go`, `ocr_*.go`, `background_sync.go`, `clustering.go`, `exif_client.go`, `main.go`, `handlers.go` | ~200 |
| exif | `exif_service.go`, `main.go`, `router.go` | ~50 |
| ocr | `classifier.go`, `image.go` | ~20 |

**Критерий приёмки:** Во всём проекте нет ни одного вызова `log.Printf` / `log.Fatalf`. Все логи структурированные JSON в stderr.

---

## 3. Приоритет 2: Производительность

### 3.1. Закэшировать GalleryAccess

**Файл (текущий):** `backend/api-service/internal/interfaces/handler/helpers/gallery.go`

**Проблема:** `IsPathInGallery()` на каждый HTTP-запрос (включая `handleServeImage`, `handleThumbnail`) выполняет `db.Find(&folders)` — полное сканирование таблицы `gallery_folders`.

**Требование:**

1. Добавить in-memory кэш с TTL:
```go
type GalleryAccess struct {
    db       *gorm.DB
    mu       sync.RWMutex
    folders  []domain.GalleryFolder
    updatedAt time.Time
    ttl      time.Duration // 30 секунд
}
```

2. Обновлять кэш при вызове `handleAddFolder` / `handleRemoveFolder` (инвалидация).

3. Если кэш пуст или истёк TTL — загрузить из БД.

**Дополнительно:** Добавить Prometheus-метрику `gallery_access_cache_hits_total`.

**Влияние:** Устраняет N+1 запросов к БД на каждый запрос изображения.

**Критерий приёмки:** `handleServeImage` выполняет 0 SQL-запросов к `gallery_folders`. Кэш обновляется при изменении списка папок.

---

### 3.2. Вынести генерацию тумбнейлов из пагинации в фон

**Файл (текущий):** `backend/api-service/internal/interfaces/handler/handlers.go` — `handleGetDuplicates`

**Проблема:** При пагинации `handleGetDuplicates` синхронно генерирует тумбнейлы для всех элементов на странице. При pageSize=500 это 500 параллельных операций декодирования+ресайза.

**Требование:**

1. Генерировать тумбнейлы асинхронно в фоновом воркере.
2. В HTTP-ответе возвращать статус генерации: `"generated"`, `"pending"`, `"failed"`.
3. Клиент (фронтенд) при получении `"pending"` повторяет запрос через polling.

**API-изменение:**
```json
{
    "thumbnail": "data:image/webp;base64,...",
    "thumbnailStatus": "generated"  // или "pending", "failed"
}
```

**Влияние:** Время ответа пагинации не зависит от количества элементов на странице.

**Критерий приёмки:** `handleGetDuplicates` отвечает за <100ms независимо от pageSize.

---

### 3.3. Пул экземпляров exiftool

**Файл (текущий):** `backend/exif/internal/application/exif_service.go`

**Проблема:** `ExtractMetadata` использует `s.et.ExtractMetadata(filePath)` — синхронный вызов. exiftool умеет обрабатывать много файлов в одном процессе.

**Требование:**

1. Создать пул экземпляров exiftool:
```go
type ExiftoolPool struct {
    pool chan *exiftool.Exiftool
}
```

2. Размер пула — из конфига (по умолчанию `runtime.NumCPU()`).

3. Добавить healthcheck для каждого экземпляра (если процесс умер — пересоздать).

**Влияние:** Параллельная обработка нескольких файлов.

**Критерий приёмки:** 10 параллельных запросов `GET /exif/metadata?path=...` обрабатываются конкурентно (не последовательно).

---

## 4. Приоритет 3: Архитектура и чистота кода

### 4.1. Убрать прямой доступ к s.db из хендлеров

**Проблема:** Хендлеры обращаются к БД напрямую через `s.db.Table(...)`, `s.db.Where(...).Find(...)` — нарушение Layer Isolation.

**Требование:**

1. Создать Repository Layer для каждого доменного агрегата:

| Repository | Методы | Используется в хендлерах |
|---|---|---|
| `DuplicateRepository` | `FindDuplicatesPaginated(page, size)`, `GetTotalCount()` | `handleGetDuplicates` |
| `ImageFileRepository` | `FindByIDs(ids)`, `DeleteByIDs(ids)`, `BatchCreate(files)` | `handleDeleteFiles`, `handleBatchDelete` |
| `GalleryFolderRepository` | `FindAll()`, `FindByID(id)`, `Create(folder)`, `Delete(id)` | `handleGetFolders`, `handleAddFolder` |
| `TrashRepository` | `GetTrashInfo()`, `ListTrash(page, size)`, `Restore(id)`, `DeletePermanently(id)` | `handle*Trash*` |
| `OCRDataRepository` | `GetDocuments(filters)`, `GetData(imageFileID)`, `GetStatus()` | `handleGetOcrDocuments`, `handleGetOcrData` |
| `SettingsRepository` | `GetAppSettings()`, `UpdateAppSettings()`, `GetUserSettings(userID)`, `UpdateUserSettings(userID)` | `handleGetSettings`, `handleUpdateSettings` |

2. Все SQL-запросы из `handler/handlers.go`, не проходящие через application-слой, переместить в соответствующие repository.

3. Добавить dependency injection репозиториев в `Server` (через Wire/fx).

**Критерий приёмки:** В папке `internal/interfaces/handler/` нет ни одного обращения к `gorm.DB`. Все запросы к БД — через repository или application-слой.

---

### 4.2. Объединить реализации thumbnail-кэша

**Проблема:** Две независимых реализации с разной семантикой:

| Реализация | Путь | Тип |
|---|---|---|
| `imaging.ThumbnailCache` | `api-service/internal/application/imaging/thumbnail.go` | In-memory map[string]string (base64) |
| `thumbnail.Service` | `api-service/internal/application/thumbnail/service.go` | Disk-кэш + service-слой + Storage interface |

**Требование:**

1. Выделить единый интерфейс:
```go
type ThumbnailProvider interface {
    GetOrGenerate(filePath string) (string, error)
    Invalidate(filePath string)
    GetStats() (*Stats, error)
}
```

2. Сделать `thumbnail.Service` единственной реализацией (on-disk).

3. `imaging.ThumbnailCache` упразднить или сделать декоратором над `thumbnail.Service` (для короткоживущего in-memory L1-кэша).

**Влияние:** Устраняет дублирование кода и несогласованность (один путь может иметь разные тумбнейлы в разных кэшах).

**Критерий приёмки:** Только одна реализация `ThumbnailProvider`. Все хендлеры используют её через единый интерфейс.

---

### 4.3. Унифицировать паттерн ретраев

**Проблема:** Клиенты (`exifclient`, `ocr/client`, `geocoder/nominatim`) имеют разные подходы к повторным попыткам:
- `exifclient.Client` — без ретраев.
- `ocr.Client` — healthcheck без ретраев.
- `nominatim.Client` — rate limiting (1s), но без ретраев.

**Требование:**

1. Создать общий пакет `internal/infrastructure/retry`:
```go
package retry

type Config struct {
    MaxAttempts int
    Delay       time.Duration
    MaxDelay    time.Duration
    Backoff     BackoffStrategy // exponential, linear, constant
}

func WithRetry[T any](ctx context.Context, cfg Config, fn func(context.Context) (T, error)) (T, error)
```

2. Интегрировать `WithRetry` во все HTTP-клиенты.

3. В `nominatim_client.go` вынести rate limiting в отдельный `RateLimitedHTTPClient`.

**Критерий приёмки:** Все вызовы внешних HTTP-сервисов имеют единообразный retry-паттерн с экспоненциальной задержкой.

---

### 4.4. Унифицировать OCR/EXIF healthcheck

**Проблема:** `ocr/client.go` и `exifclient/client.go` имеют дублированную логику healthcheck:
- `StartHealthCheck(interval)` / `StopHealthCheck()` / `IsHealthy()`.
- Оба реализованы через time.Ticker.

**Требование:**

1. Создать общий пакет `internal/infrastructure/healthcheck`:
```go
type Checker interface {
    Check(ctx context.Context) error
}

type PeriodicChecker struct {
    mu       sync.RWMutex
    healthy  bool
    lastErr  error
    stopCh   chan struct{}
}

func NewPeriodicChecker(checker Checker, interval time.Duration) *PeriodicChecker
func (pc *PeriodicChecker) Start(ctx context.Context)
func (pc *PeriodicChecker) Stop()
func (pc *PeriodicChecker) IsHealthy() bool
```

2. Реализовать `ExifHealthChecker` и `OCRHealthChecker`, реализующие `Checker`.

3. Удалить дублированную логику из `ocr/client.go` и `exifclient/client.go`.

**Критерий приёмки:** В `ocr/client.go` и `exifclient/client.go` нет кода, связанного с healthcheck.

---

## 5. Приоритет 4: Developer Experience и CI

### 5.1. Добавить make test-all

**Текущее состояние:** Нужно вручную запускать `go test ./...` в каждом сервисе.

**Требование:** Добавить цели в `Makefile`:

```makefile
.PHONY: test test-api test-exif test-ocr test-all

test-api:
    cd backend/api-service && go test ./... -v -count=1 -timeout=120s | tail -20

test-exif:
    cd backend/exif && go test ./... -v -count=1 -timeout=60s | tail -20

test-ocr:
    cd backend/ocr && go test ./... -v -count=1 -timeout=120s | tail -20

test-all: test-api test-exif test-ocr

test-race:
    cd backend/api-service && go test -race ./...
```

**Критерий приёмки:** `make test-all` запускает тесты всех трёх сервисов и завершается с 0 exit code.

---

### 5.2. Добавить golangci-lint

**Требование:**

1. Создать `.golangci.yml` в корне проекта:
```yaml
linters:
  enable:
    - errcheck
    - gosimple
    - govet
    - ineffassign
    - staticcheck
    - revive
    - goconst
    - gocyclo
    - gofmt
    - misspell
    - unconvert
    - prealloc
  disable:
    - unused

issues:
  max-per-linter: 50
  exclude-use-default: false

run:
  timeout: 3m
  tests: true
```

2. Добавить цели в `Makefile`:
```makefile
.PHONY: lint lint-fix

lint:
    cd backend/api-service && golangci-lint run ./...
    cd backend/exif && golangci-lint run ./...
    cd backend/ocr && golangci-lint run ./...

lint-fix:
    cd backend && golangci-lint run --fix ./...
```

3. Добавить `golangci-lint` в CI/CD.

**Критерий приёмки:** `golangci-lint run ./...` проходит во всех трёх сервисах с 0 ошибками.

---

### 5.3. Добавить make docker-build-all

**Требование:** Добавить цели для сборки Docker-образов:

```makefile
DOCKER_IMAGE ?= flashbacks
DOCKER_TAG ?= latest

.PHONY: docker-build docker-build-api docker-build-exif docker-build-ocr docker-build-all

docker-build-all:
    docker compose build

docker-build-api:
    docker build --target=api -t $(DOCKER_IMAGE)/api:$(DOCKER_TAG) -f backend/Dockerfile backend/

docker-build-exif:
    docker build --target=exif -t $(DOCKER_IMAGE)/exif:$(DOCKER_TAG) -f backend/Dockerfile backend/

docker-build-ocr:
    docker build --target=ocr -t $(DOCKER_IMAGE)/ocr:$(DOCKER_TAG) -f backend/Dockerfile backend/

docker-push:
    docker push $(DOCKER_IMAGE)/api:$(DOCKER_TAG)
    docker push $(DOCKER_IMAGE)/exif:$(DOCKER_TAG)
    docker push $(DOCKER_IMAGE)/ocr:$(DOCKER_TAG)
```

**Критерий приёмки:** `make docker-build-all` собирает все три образа без ошибок.

---

## Приложение A: Matrix зависимостей

### DI-граф api-service (текущий)

```
main.go
 ├── config.LoadConfig()
 ├── database.InitDatabase(cfg)
 ├── exifclient.NewHTTPExifClient(url)
 ├── geocoder.NewNominatimClient()
 ├── geocoder.NewGeolocationService(db, nominatim)
 ├── ocr.NewClient(url)
 ├── imaging.NewScanManager(db, workers)
 ├── imaging.NewOcrManager(db, ocrClient, workers)
 ├── imaging.NewLlmOcrService(db)
 ├── imaging.NewBackgroundSyncManager(db, thumb, geo, exifClient)
 ├── imaging.NewTagScanManager(db, llmOcr, maxMP)
 ├── imaging.NewEmbeddingBackfillManager(db)
 ├── thumbnail.NewService(cfg)
 ├── auth.NewSessionConfig()
 ├── auth.NewSessionRepository(db, sessionConfig)
 ├── auth.NewBootstrapService(db, login, pass)
 ├── auth.NewLoginRateLimiter(...)
 ├── auth.NewAuthService(db, bootstrap, sessionRepo, limiter)
 ├── auth.NewUserService(db, sessionRepo)
 ├── auth.NewSessionCleanupJob(sessionRepo, interval)
 ├── middleware.NewAuthMiddleware(sessionRepo, auth, i18n)
 ├── middleware.NewCSRFProtection(i18n)
 ├── handler.NewAuthHandlers(auth, bootstrap, user, sessionRepo, db, i18n)
 ├── helpers.NewLLMFactory(db, maxMP)
 ├── mcpserver.NewFlashbacksMCPServer(db, llmFactory, llmOcr, maxMP, embedding, exifAgent)
 ├── agent.NewExifAgent(exifURL, backupDir)
 ├── agent.NewConversationService(db)
 ├── agent.NewAgent(convService, mcpSrv, cfg)
 └── handler.NewServer(db, scanMgr, ocrMgr, llmOcr, bgSync, tagScan, embedding, thumb, cfg, geo, nominatim, mcpSrv, agent, agentCfg, convSvc, exifClient)
     └── server.SetupRouter(authMiddleware, csrf, authHandlers)
```

### DI-граф exif (текущий)

```
main.go
 ├── config.Load()
 ├── database.Init(cfg)
 ├── application.NewMetadataRepo(db)
 ├── application.NewNominatimClient()
 ├── go-exiftool.NewExiftool()
 ├── application.NewExifService(et, repo, nominatim)
 ├── application.NewGPSWriter()
 ├── exifmcp.NewHTTPHandler(exifSvc, gpsWriter)
 └── handler.SetupRouter(db, exifSvc, gpsWriter)
```

### DI-граф ocr (текущий)

```
main.go
 ├── config.Load()
 ├── handler.NewClassifyHandler()
 └── http.Server
```