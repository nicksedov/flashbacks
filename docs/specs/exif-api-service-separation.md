# Spec: Разделение микросервисов `api-service` и `exif`

> **Статус:** Черновик
> **Дата:** 2026-06-27
> **Автор:** Zoo (AI Agent)

## Цель

Устранить дублирование структур `ImageMetadata` и `GeolocationCache` между микросервисами `backend/api-service` и `backend/exif`. Целевая архитектура: структуры располагаются **только в домене exif**, а `api-service` при необходимости обращается за получением этих данных через REST-запросы к `exif`.

---

## 1. Текущее состояние (анализ)

### 1.1. Дублирующиеся структуры

| Структура | api-service | exif |
|-----------|-------------|------|
| [`ImageMetadata`](../backend/api-service/internal/domain/media.go#L57) | Строки 57–75 (GORM-модель, 19 полей) | [Строки 7–25](../backend/exif/internal/domain/media.go#L7) (идентично) |
| [`GeolocationCache`](../backend/api-service/internal/domain/media.go#L77) | Строки 77–85 (GORM-модель, 5 полей) | [Строки 28–34](../backend/exif/internal/domain/media.go#L28) (идентично) |

Обе структуры побайтово идентичны, включая GORM-теги и JSON-теги.

### 1.2. Кто владеет схемой БД

- **api-service** — выполняет `AutoMigrate` для `image_metadata` и `geolocation_caches` в [`database.go:41-42`](../backend/api-service/internal/infrastructure/database/database.go#L41)
- **exif** — выполняет `AutoMigrate` как «проверку совместимости схемы» в [`database.go:32`](../backend/exif/internal/infrastructure/database/database.go#L32), комментарий гласит: *"The EXIF service does NOT run AutoMigrate — schema is managed by the main backend"* (но фактически `AutoMigrate` вызывается)
- Оба сервиса подключаются к **одной и той же PostgreSQL БД** (`image_toolkit`)

### 1.3. Точки прямого доступа api-service к таблицам `image_metadata` и `geolocation_caches`

| # | Операция | Файл:строка | Тип |
|---|----------|-------------|-----|
| 1 | Сохранение метаданных (upsert по `image_file_id`) | [`background_sync.go:571`](../backend/api-service/internal/application/imaging/background_sync.go#L571) | Write |
| 2 | Получение метаданных по `image_file_id` | [`handlers.go:1054`](../backend/api-service/internal/interfaces/handler/handlers.go#L1054) | Read |
| 3 | Поиск изображений без EXIF (missing `date_taken` OR `geolocation_ref`) | [`handlers.go:1118-1133`](../backend/api-service/internal/interfaces/handler/handlers.go#L1118) | Read (JOIN) |
| 4 | Календарная галерея (`date_taken` grouping + cursor pagination) | [`handlers.go:1178+`](../backend/api-service/internal/interfaces/handler/handlers.go#L1178) | Read (JOIN) |
| 5 | Location candidates (same-day GPS via `geolocation_caches` JOIN) | [`handlers.go:3311-3314`](../backend/api-service/internal/interfaces/handler/handlers.go#L3311) | Read (JOIN) |
| 6 | Геокластеризация (GPS bounds + JOIN `image_files→metadata→geolocation_caches`) | [`clustering.go:93-96`](../backend/api-service/internal/application/geo/clustering.go#L93) | Read (JOIN) |
| 7 | Каскадное удаление метаданных при удалении `ImageFile` | [`scanner.go:584`](../backend/api-service/internal/application/imaging/scanner.go#L584) | Delete |
| 8 | Пакетное обновление GPS (upsert `image_metadata` с `geolocation_ref`) | [`handlers.go:3462-3485`](../backend/api-service/internal/interfaces/handler/handlers.go#L3462) | Write |
| 9 | Обогащение метаданных из файла + сохранение в БД | [`handlers.go:3459`](../backend/api-service/internal/interfaces/handler/handlers.go#L3459) | Read (через exifClient) + Write |
| 10 | Разрешение геолокации (`GeolocationCache` read/write) | [`geolocation_service.go:36-101`](../backend/api-service/internal/infrastructure/geocoder/geolocation_service.go#L36) | Read/Write |
| 11 | MCP tools: чтение геолокации из кэша | [`tools_search.go:424-426`](../backend/api-service/internal/infrastructure/mcpserver/tools_search.go#L424) | Read |
| 12 | Календарь: `MIN(date_taken)`, `MAX(date_taken)` | [`handlers.go:1452`](../backend/api-service/internal/interfaces/handler/handlers.go#L1452) | Read (aggregate) |
| 13 | Календарь: `DISTINCT EXTRACT(DAY FROM date_taken)` за месяц | [`handlers.go:1471-1476`](../backend/api-service/internal/interfaces/handler/handlers.go#L1471) | Read (DISTINCT) |

### 1.4. Существующие контракты EXIF (уже используются или доступны)

| Метод | Путь | Используется api-service? | Примечание |
|-------|------|--------------------------|------------|
| GET | `/exif/health` | Да | Health check ([`client.go:227`](../backend/api-service/internal/infrastructure/exifclient/client.go#L227)) |
| GET | `/exif/metadata?path=` | Да | Извлечение метаданных из файла ([`client.go:32`](../backend/api-service/internal/infrastructure/exifclient/client.go#L32)) |
| PUT | `/exif/gps` | Да | Запись GPS в файл ([`client.go:130`](../backend/api-service/internal/infrastructure/exifclient/client.go#L130)) |
| PUT | `/exif/gps/batch` | Нет | — |
| GET | `/exif/location-candidates?date=` | **Нет!** | api-service дублирует логику локально ([`handlers.go:3287-3382`](../backend/api-service/internal/interfaces/handler/handlers.go#L3287)) вместо вызова exif ([`router.go:145`](../backend/exif/internal/interfaces/handler/router.go#L145)) |
| GET | `/exif/missing` | Нет | api-service делает свой локальный запрос |

**Ключевая находка:** api-service дублирует логику `/exif/location-candidates` и `/exif/missing` локально, хотя EXIF-сервис уже предоставляет эти эндпоинты.

---

## 2. Целевая архитектура

```
┌─────────────────────────────────────────────────────────┐
│ api-service                                             │
│  ┌──────────┐  ┌──────────────┐  ┌──────────────────┐  │
│  │ handlers │  │ background   │  │ geo/clustering   │  │
│  │ (REST)   │  │ sync/scanner │  │                  │  │
│  └────┬─────┘  └──────┬───────┘  └────────┬─────────┘  │
│       │               │                   │             │
│       └───────────────┼───────────────────┘             │
│                       │                                 │
│               ┌───────▼────────┐                        │
│               │  ExifClient    │  (расширенный)          │
│               │  (HTTP client) │                        │
│               └───────┬────────┘                        │
│                       │ HTTP                            │
│       ════════════════╪══════════════════════════       │
│                       ▼                                 │
│ ┌─────────────────────────────────────────────────┐     │
│ │ exif service (владелец image_metadata +         │     │
│ │             geolocation_caches)                 │     │
│ │                                                 │     │
│ │  REST API:                                      │     │
│ │  GET  /exif/metadata         (file + DB)        │     │
│ │  PUT  /exif/metadata         (upsert)     NEW   │     │
│ │  DELETE /exif/metadata/:id   (cascade)    NEW   │     │
│ │  GET  /exif/metadata/batch   (multi-ID)   NEW   │     │
│ │  GET  /exif/metadata/calendar          NEW      │     │
│ │  GET  /exif/metadata/geo-points        NEW      │     │
│ │  GET  /exif/missing           (paginated) EXTEND │     │
│ │  GET  /exif/location-candidates (exists)        │     │
│ │  GET  /exif/geolocation       (resolve)   NEW   │     │
│ │  PUT  /exif/gps               (exists)          │     │
│ │  PUT  /exif/gps/batch         (exists)          │     │
│ │                                                 │     │
│ │         ┌──────────┐                            │     │
│ │         │ PostgreSQL│  (image_metadata,          │     │
│ │         │          │   geolocation_caches)      │     │
│ │         └──────────┘                            │     │
│ └─────────────────────────────────────────────────┘     │
│                                                         │
│  api-service владеет:                                   │
│  ┌──────────┐                                           │
│  │ PostgreSQL│  (image_files, ocr_*, llm_*,             │
│  │          │   image_tags, tag_embeddings,             │
│  │          │   users, sessions, conversations, etc.)   │
│  └──────────┘                                           │
└─────────────────────────────────────────────────────────┘
```

---

## 3. План реализации

### Фаза 1: Устранение дублирования + передача владения схемой

**Цель:** Убрать дубликаты `ImageMetadata` и `GeolocationCache`, передать владение миграциями EXIF-сервису.

| # | Действие | Файлы | Описание |
|---|----------|-------|----------|
| 1.1 | Создать `backend/shared/domain/` — общий Go-модуль | `backend/shared/go.mod`, `backend/shared/domain/media.go` | Вынести `ImageMetadata` и `GeolocationCache` со всеми GORM/JSON-тегами в общий пакет |
| 1.2 | Обновить `go.mod` в api-service и exif | `backend/api-service/go.mod`, `backend/exif/go.mod` | Добавить `require` + `replace` директиву на shared-модуль |
| 1.3 | Удалить дубликаты из api-service | [`media.go`](../backend/api-service/internal/domain/media.go) | Убрать строки 57–85 (`ImageMetadata` + `GeolocationCache`), заменить на импорт из shared |
| 1.4 | Удалить дубликаты из exif | [`media.go`](../backend/exif/internal/domain/media.go) | Заменить на импорт из shared |
| 1.5 | Передать владение `AutoMigrate` exif-сервису | [`database.go`](../backend/exif/internal/infrastructure/database/database.go#L32) | Убрать комментарий о «проверке совместимости», оставить полноценный `AutoMigrate` |
| 1.6 | Убрать `AutoMigrate` из api-service | [`database.go`](../backend/api-service/internal/infrastructure/database/database.go#L41) | Удалить `&domain.ImageMetadata{}` и `&domain.GeolocationCache{}` из списка миграций |
| 1.7 | Запустить тесты обоих сервисов | — | `go test ./internal/application/... -count=1` |

**Результат фазы 1:** Дублирование устранено, но api-service всё ещё ходит в БД напрямую. Безопасная промежуточная точка.

---

### Фаза 2: Новые/расширенные API-контракты EXIF-сервиса

**Цель:** Создать REST-эндпоинты в EXIF-сервисе, покрывающие все потребности api-service.

#### 2.1. Расширение существующих контрактов

| Контракт | Изменение | Потребность api-service |
|----------|-----------|------------------------|
| `GET /exif/metadata` | Добавить опциональный `imageFileId` (query param). Если передан — читать из БД, если `path` — из файла. | Замена [`handlers.go:1054`](../backend/api-service/internal/interfaces/handler/handlers.go#L1054) |
| `GET /exif/missing` | Добавить пагинацию (`page`, `pageSize`), возвращать `{images: [...], total: N}` | Замена [`handlers.go:1118-1133`](../backend/api-service/internal/interfaces/handler/handlers.go#L1118) |
| `GET /exif/location-candidates` | **Без изменений.** Просто начать использовать из api-service. | Замена [`handlers.go:3287-3382`](../backend/api-service/internal/interfaces/handler/handlers.go#L3287) |

#### 2.2. Новые контракты

##### `PUT /exif/metadata` — Upsert метаданных

Запрос:
```json
{
  "imageFileId": 123,
  "width": 4032,
  "height": 3024,
  "cameraModel": "Canon EOS R5",
  "lensModel": "RF 24-70mm F2.8",
  "iso": 400,
  "aperture": "f/2.8",
  "shutterSpeed": "1/250s",
  "focalLength": "50mm",
  "dateTaken": "2025-06-15T14:30:00Z",
  "orientation": 1,
  "colorSpace": "sRGB",
  "software": "Adobe Photoshop 25.0",
  "geolocationRef": 42
}
```
Ответ `200`:
```json
{
  "id": 456,
  "imageFileId": 123,
  "width": 4032,
  "...": "..."
}
```
Назначение: замена [`background_sync.go:571`](../backend/api-service/internal/application/imaging/background_sync.go#L571).

##### `DELETE /exif/metadata/:imageFileId` — Каскадное удаление

Ответ `200`:
```json
{ "deleted": true }
```
Назначение: замена [`scanner.go:584`](../backend/api-service/internal/application/imaging/scanner.go#L584).

##### `GET /exif/metadata/batch?ids=1,2,3` — Пакетное получение

Ответ `200`:
```json
{
  "metadata": [
    { "imageFileId": 1, "width": 4032, "...": "..." },
    { "imageFileId": 2, "width": 1920, "...": "..." }
  ]
}
```
Назначение: получение метаданных для списка изображений (галерея).

##### `GET /exif/metadata/calendar` — Календарная галерея

Параметры:
| Параметр | Тип | Обязательный | Описание |
|----------|-----|-------------|----------|
| `startDate` | string (YYYY-MM-DD) | Нет | Начало диапазона |
| `endDate` | string (YYYY-MM-DD) | Нет | Конец диапазона |
| `pageSize` | int | Нет | Размер страницы (default: 30) |
| `cursor` | string (base64) | Нет | Курсор пагинации |

Ответ `200`:
```json
{
  "items": [
    {
      "imageFileId": 1,
      "path": "/photos/img001.jpg",
      "dateTaken": "2025-06-15T14:30:00Z",
      "geolocationRef": 42,
      "gpsLatitude": 55.7558,
      "gpsLongitude": 37.6173,
      "nameLocal": "Москва",
      "nameEng": "Moscow"
    }
  ],
  "nextCursor": "base64cursor",
  "dateRange": {
    "minDate": "2024-01-01",
    "maxDate": "2025-12-31"
  },
  "totalWithDate": 1500
}
```
Назначение: замена [`handlers.go:1178+`](../backend/api-service/internal/interfaces/handler/handlers.go#L1178).

##### `GET /exif/metadata/geo-points` — GPS-точки для кластеризации

Параметры:
| Параметр | Тип | Обязательный | Описание |
|----------|-----|-------------|----------|
| `minLat` | float64 | Да* | Южная граница |
| `maxLat` | float64 | Да* | Северная граница |
| `minLng` | float64 | Да* | Западная граница |
| `maxLng` | float64 | Да* | Восточная граница |

> *Все четыре параметра обязательны одновременно. Если не указаны — возвращаются все точки.

Ответ `200`:
```json
{
  "points": [
    {
      "imageFileId": 1,
      "path": "/photos/img001.jpg",
      "gpsLatitude": 55.7558,
      "gpsLongitude": 37.6173,
      "nameLocal": "Москва",
      "nameEng": "Moscow"
    }
  ]
}
```
Назначение: замена JOIN-запроса в [`clustering.go:93-96`](../backend/api-service/internal/application/geo/clustering.go#L93).

##### `GET /exif/geolocation` — Разрешение геолокации

Параметры:
| Параметр | Тип | Обязательный | Описание |
|----------|-----|-------------|----------|
| `lat` | float64 | Да | Широта |
| `lng` | float64 | Да | Долгота |

Ответ `200`:
```json
{
  "id": 42,
  "gpsLatitude": 55.7558,
  "gpsLongitude": 37.6173,
  "nameLocal": "Москва, Центральный округ",
  "nameEng": "Moscow, Central District"
}
```
Логика: проверка кэша → вызов Nominatim (с rate-limiting) → сохранение в кэш → возврат.
Назначение: замена [`geolocation_service.go:36-101`](../backend/api-service/internal/infrastructure/geocoder/geolocation_service.go#L36).

#### 2.3. Реализация в EXIF-сервисе

| # | Файл | Описание |
|---|------|----------|
| 2.3.1 | [`dto/metadata_dto.go`](../backend/exif/internal/interfaces/dto/metadata_dto.go) | Добавить DTO: `UpsertMetadataRequest`, `MetadataBatchResponse`, `CalendarResponse`, `CalendarItem`, `GeoPointItem`, `GeoPointsResponse`, `GeolocationResponse`, `MissingImagesResponse` |
| 2.3.2 | Новый: `application/metadata_repo.go` | Репозиторий для CRUD-операций с `image_metadata` и `geolocation_caches` через GORM |
| 2.3.3 | [`application/exif_service.go`](../backend/exif/internal/application/exif_service.go) | Добавить методы: `GetMetadataByImageID`, `UpsertMetadata`, `DeleteMetadata`, `GetMissingImages`, `GetCalendarItems`, `GetGeoPoints`, `ResolveGeolocation` |
| 2.3.4 | [`handler/router.go`](../backend/exif/internal/interfaces/handler/router.go) | Добавить новые маршруты, расширить `HandleGetMetadata` |
| 2.3.5 | Новый: `application/nominatim_client.go` | Перенести Nominatim-клиент из api-service в exif (сейчас он в [`geocoder/nominatim.go`](../backend/api-service/internal/infrastructure/geocoder/nominatim.go)) |
| 2.3.6 | [`docs/api-contracts/exif.yaml`](../docs/api-contracts/exif.yaml) | Обновить OpenAPI-спеку |

---

### Фаза 3: Миграция api-service на вызовы EXIF API

**Цель:** Убрать все прямые обращения api-service к таблицам `image_metadata` и `geolocation_caches`.

#### 3.1. Расширение интерфейса `ExifClient`

В [`exif_client.go`](../backend/api-service/internal/application/imaging/exif_client.go#L10):

```go
type ExifClient interface {
    // Существующие (сохраняются)
    ExtractMetadata(ctx context.Context, filePath string) (*domain.ImageMetadata, error)
    ExtractGPS(ctx context.Context, filePath string) (lat, lng float64, ok bool, err error)
    WriteGPS(ctx context.Context, filePath string, lat, lng float64, backupDir string, meta *domain.ImageMetadata) error
    EnrichMissingMetadata(ctx context.Context, filePath string, meta *domain.ImageMetadata) (map[string]interface{}, error)
    Health(ctx context.Context) (*domain.ExifHealthStatus, error)

    // Новые — метаданные
    GetMetadataByImageID(ctx context.Context, imageFileID uint) (*domain.ImageMetadata, error)
    UpsertMetadata(ctx context.Context, meta *domain.ImageMetadata) (*domain.ImageMetadata, error)
    DeleteMetadata(ctx context.Context, imageFileID uint) error
    GetMetadataBatch(ctx context.Context, imageFileIDs []uint) (map[uint]*domain.ImageMetadata, error)
    GetCalendarItems(ctx context.Context, params CalendarParams) (*CalendarResult, error)
    GetGeoPoints(ctx context.Context, bounds GeoBounds) ([]GeoPoint, error)

    // Новые — missing / location-candidates
    GetMissingImages(ctx context.Context, page, pageSize int) (*MissingImagesResult, error)
    GetLocationCandidates(ctx context.Context, date string) ([]LocationCandidate, error)

    // Новые — геолокация
    ResolveGeolocation(ctx context.Context, lat, lng float64) (*domain.GeolocationCache, error)
}
```

#### 3.2. Пошаговая замена

| # | Файл(ы) в api-service | Что заменить | Комментарий |
|---|----------------------|--------------|-------------|
| 3.2.1 | [`exifclient/client.go`](../backend/api-service/internal/infrastructure/exifclient/client.go) | Реализовать все новые методы HTTP-клиента | Самый объёмный шаг |
| 3.2.2 | [`geolocation_service.go`](../backend/api-service/internal/infrastructure/geocoder/geolocation_service.go) | `ResolveGeolocation` → вызов `exifClient.ResolveGeolocation`. Удалить прямой доступ к `geolocation_caches`. | После этого `NominatimClient` и `GeolocationService` могут быть удалены из api-service |
| 3.2.3 | [`background_sync.go:571`](../backend/api-service/internal/application/imaging/background_sync.go#L571) | `db.Where(...).Assign(meta).FirstOrCreate(...)` → `exifClient.UpsertMetadata(ctx, meta)` | — |
| 3.2.4 | [`scanner.go:584`](../backend/api-service/internal/application/imaging/scanner.go#L584) | `db.Where(...).Delete(&domain.ImageMetadata{})` → `exifClient.DeleteMetadata(ctx, imageFileID)` | — |
| 3.2.5 | [`handlers.go:1054-1073`](../backend/api-service/internal/interfaces/handler/handlers.go#L1054) | Прямой запрос `image_metadata` + `geolocation_caches` → `exifClient.GetMetadataByImageID(ctx, id)` | Ответ должен включать GPS-данные |
| 3.2.6 | [`handlers.go:1118-1133`](../backend/api-service/internal/interfaces/handler/handlers.go#L1118) | `handleGetImagesMissingExif` → `exifClient.GetMissingImages(ctx, page, pageSize)` | EXIF-сервис должен вернуть `{imageFileId, path, missingDate, missingGps}` |
| 3.2.7 | [`handlers.go:3287-3382`](../backend/api-service/internal/interfaces/handler/handlers.go#L3287) | `handleGetLocationCandidates` → `exifClient.GetLocationCandidates(ctx, date)` | Логика уже реализована в exif |
| 3.2.8 | [`handlers.go:1178+`](../backend/api-service/internal/interfaces/handler/handlers.go#L1178) | Календарная галерея → `exifClient.GetCalendarItems(ctx, params)`. Группировка по дням и миниатюры остаются в api-service. | EXIF отдаёт плоский список |
| 3.2.9 | [`clustering.go:93-96`](../backend/api-service/internal/application/geo/clustering.go#L93) | JOIN-запрос → `exifClient.GetGeoPoints(ctx, bounds)`, затем локальная кластеризация | Сама кластеризация (gocluster) остаётся в api-service |
| 3.2.10 | [`handlers.go:3462-3485`](../backend/api-service/internal/interfaces/handler/handlers.go#L3462) | `db.Create/Update` → `exifClient.UpsertMetadata(ctx, &meta)` | После `WriteGPS` и `EnrichMissingMetadata` |
| 3.2.11 | [`tools_search.go:424-426`](../backend/api-service/internal/infrastructure/mcpserver/tools_search.go#L424) | Прямой запрос `geolocation_caches` → использовать данные из `exifClient.GetMetadataByImageID` | — |
| 3.2.12 | [`testutil/mocks/mock_exif.go`](../backend/api-service/internal/testutil/mocks/mock_exif.go) | Добавить моки для всех новых методов | — |
| 3.2.13 | [`database.go`](../backend/api-service/internal/infrastructure/database/database.go#L64) | Убрать `CREATE INDEX IF NOT EXISTS idx_image_metadata_date_taken_file_id` — индекс теперь управляется exif | — |
| 3.2.14 | Все тесты | Обновить тесты, использующие прямые запросы к `image_metadata`/`geolocation_caches` | — |

---

## 4. Риски и альтернативные решения

### 4.1. Геокластеризация (`GET /exif/metadata/geo-points`)

**Проблема:** При большом количестве изображений с GPS (тысячи) передача всех точек через HTTP может быть дорогой.

**Альтернативы:**
- **A)** Оставить geo-запрос в api-service как read-only (только SELECT, без миграций).
- **B)** Перенести саму кластеризацию в EXIF-сервис: `GET /exif/metadata/clusters?minLat=...&maxLat=...&zoom=...`. Тогда api-service получает уже готовые кластеры.
- **C)** Использовать материализованное представление или реплику для read-only доступа.

**Рекомендация:** **Вариант B** — наиболее чистый архитектурно. Библиотека `gocluster` переносится в exif-сервис.

### 4.2. Календарная галерея (`GET /exif/metadata/calendar`)

**Проблема:** Самый сложный запрос: группировка по датам, курсорная пагинация с пересечением границ дней, параллельная генерация миниатюр.

**Решение в плане:** EXIF возвращает **плоский список** `{imageFileId, path, dateTaken, geolocationRef, gpsLatitude, gpsLongitude, nameLocal, nameEng}` для заданного диапазона. Группировка по дням и генерация миниатюр остаются в api-service. Это снижает сложность контракта.

### 4.3. Миграция данных / порядок запуска

**Проблема:** Оба сервиса мигрируют одни и те же таблицы. Нужно гарантировать, что exif мигрирует **до** api-service.

**Решение:** В `docker-compose.yml` добавить:
```yaml
api-service:
  depends_on:
    exif:
      condition: service_healthy
```

### 4.4. `GeolocationService` и `NominatimClient` в api-service

После Фазы 3 эти компоненты станут ненужными в api-service:
- [`geocoder/geolocation_service.go`](../backend/api-service/internal/infrastructure/geocoder/geolocation_service.go) — удалить
- [`geocoder/nominatim.go`](../backend/api-service/internal/infrastructure/geocoder/nominatim.go) — удалить (перенести в exif)
- [`geocoder/nominatim_test.go`](../backend/api-service/internal/infrastructure/geocoder/nominatim_test.go) — удалить (перенести в exif)

---

## 5. Оценка трудозатрат

| Фаза | Шагов | Файлов | Оценка (дней) |
|------|-------|--------|---------------|
| Фаза 1: Shared-модуль + передача владения | 7 | ~8 | 1–2 |
| Фаза 2: Новые API-контракты EXIF | 6 | ~6 новых/изменённых | 3–5 |
| Фаза 3: Миграция api-service | 14 | ~15 изменённых | 5–8 |
| **Итого** | **27** | **~29** | **9–15** |

---

## 6. Порядок выполнения

```
Фаза 1 ──► Фаза 2 ──► Фаза 3
```

Фазы строго последовательны. Внутри Фазы 2 и 3 шаги могут выполняться параллельно разными разработчиками.

**Рекомендация:** Каждый шаг Фазы 3 оформлять отдельным PR для минимизации рисков и облегчения code review.

---

## 7. Критерии приёмки

- [ ] В `api-service/internal/domain/media.go` отсутствуют структуры `ImageMetadata` и `GeolocationCache`
- [ ] В `exif/internal/domain/media.go` отсутствуют дубликаты (импорт из shared)
- [ ] `AutoMigrate` для `image_metadata` и `geolocation_caches` выполняется только в exif-сервисе
- [ ] Все прямые GORM-запросы api-service к `image_metadata` и `geolocation_caches` заменены на вызовы `ExifClient`
- [ ] Все существующие тесты проходят (api-service и exif)
- [ ] Новые тесты покрывают добавленные эндпоинты EXIF-сервиса
- [ ] OpenAPI-спека [`exif.yaml`](../docs/api-contracts/exif.yaml) актуализирована
- [ ] `docker-compose.yml` содержит `depends_on: exif` для api-service
