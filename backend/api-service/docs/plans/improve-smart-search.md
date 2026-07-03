# План: Улучшение качества умного поиска (Smart Search)

## Текущая реализация

- [`smart_search.go`](../../internal/application/imaging/smart_search.go) — одна функция `SearchByEmbedding()`:
  1. Векторизация поискового запроса через LLM embedding provider
  2. Поиск по косинусной близости в pgvector (`<=>` оператор) по таблицам `tag_embeddings_<model>`
  3. Дедупликация по `image_file_id` (один тег → один вектор)
  4. Возврат результатов, отсортированных по расстоянию (ближайшие сверху)
- [`handlers_smart_search.go`](../../internal/interfaces/handler/handlers_smart_search.go) — HTTP handler:
  - `GET /api/gallery/smart-search?q=...&limit=20`
  - Дефолтный лимит: **20**, максимум: **100**
- [`useSmartSearch.ts`](../../../webapp/src/hooks/useSmartSearch.ts) — фронтенд-хук с дефолтным лимитом **20**
- [`SmartSearchTab.tsx`](../../../webapp/src/components/tabs/SmartSearchTab.tsx) — UI с дефолтным лимитом **50**
- MCP сервер также использует `SearchByEmbedding()` через `querySemanticSearch()`

---

## Желаемая реализация

### 1. Два параллельных поисковых потока

#### Поток A — Поиск по полному совпадению тега (`SearchByExactTag`)
- Поиск в таблице `image_tags` по точному совпадению `tag` = поисковая строка
- С учётом регистра и количества пробелов (`=`, не `ILIKE`)
- `SELECT DISTINCT image_file_id FROM image_tags WHERE tag = ?`
- Результаты получают `Similarity = 1.0` (максимальная)

#### Поток B — Поиск по эмбеддингам (`SearchByEmbedding`)
- Существующая реализация (без изменений в логике поиска)

#### Параллельное выполнение
- Использовать `errgroup` или `sync.WaitGroup` для запуска двух горутин
- Таймаут на каждый поток (например, 30s) через контекст

### 2. Объединение и ранжирование результатов (`MergeResults`)

Новая функция, которая получает результаты обоих потоков и возвращает единый список.

#### Правила ранжирования (по убыванию приоритета):
1. **Дубликаты (оба потока)** — изображения, найденные и по тегу, и по эмбеддингам. Ранжируются выше всех. `Similarity = 1.5` (бустинг).
2. **Только точный тег** — `Similarity = 1.0`
3. **Только эмбеддинги** — `Similarity = оригинальное значение (0.0–1.0)`

#### Дедупликация:
- Устраняются дубликаты `image_file_id` между потоками
- Если изображение найдено в обоих потоках — сохраняется одна запись с повышенным рангом (правило 1)

#### Сохранение порядка:
- Сначала изображения из правила 1 (отсортированы по similarity эмбеддингов)
- Затем изображения из правила 2 (точный тег, порядок не важен)
- Затем изображения из правила 3 (отсортированы по similarity эмбеддингов)

### 3. Увеличение лимита по умолчанию до 100

- В Go handler: `DefaultQuery("limit", "100")`, max оставить 200
- Во фронтенд-хуке `useSmartSearch`: дефолт `limit = 100`
- В `SmartSearchTab`: дефолт `setLimit(100)`, max в UI поднять до 200
- В API-контракте: обновить default
- В MCP `clampLimit`: поднять до 200

---

## Изменяемые файлы

### Backend (Go)

| Файл | Изменения |
|---|---|
| `backend/api-service/internal/application/imaging/smart_search.go` | Добавить `SearchByExactTag()`, `MergeAndRankResults()`, обновить `SmartSearchResult` (добавить `MatchType`), изменить `SearchByEmbedding` сигнатуру при необходимости |
| `backend/api-service/internal/interfaces/handler/handlers_smart_search.go` | Изменить `handleSmartSearch` на параллельный запуск, увеличить дефолтный лимит до 100, max до 200 |
| `backend/api-service/internal/interfaces/handler/router.go` | Без изменений |
| `backend/api-service/internal/infrastructure/mcpserver/tools_search.go` | Обновить `querySemanticSearch` (возможно, оставить как есть — MCP для AI агента, ему точный тег не нужен) |
| `backend/api-service/internal/interfaces/i18n/messages.go` | При необходимости — добавить новые сообщения об ошибках |
| `backend/api-service/internal/interfaces/i18n/locales/en.json` | Переводы |
| `backend/api-service/internal/interfaces/i18n/locales/ru.json` | Переводы |

### Frontend (TypeScript)

| Файл | Изменения |
|---|---|
| `webapp/src/api/endpoints.ts` | Дефолтный `limit = 100` в `smartSearch()` |
| `webapp/src/hooks/useSmartSearch.ts` | Дефолтный `limit = 100` в `search()` |
| `webapp/src/components/tabs/SmartSearchTab.tsx` | Дефолтный `setLimit(100)`, max с 200 до 200 (оставить) |
| `webapp/src/types/index.ts` | Добавить опциональное поле `matchType?: "exact" \| "embedding" \| "both"` в `SmartSearchResult` |

### Документация

| Файл | Изменения |
|---|---|
| `docs/api-contracts/api-service.yaml` | Обновить дефолтный limit с 50 до 100, добавить `matchType` в `SmartSearchResponse`/`SearchResult` |

---

## Детальный план реализации

### Шаг 1: Подготовка типов

В [`smart_search.go`](../../internal/application/imaging/smart_search.go):

```go
// MatchType indicates how a result was matched.
type MatchType string

const (
    MatchExact     MatchType = "exact"
    MatchEmbedding MatchType = "embedding"
    MatchBoth      MatchType = "both"
)

// SmartSearchResult — добавить поле MatchType
type SmartSearchResult struct {
    ImageFileID uint
    Path        string
    ModTime     time.Time
    Similarity  float64
    Tags        []string
    MatchType   MatchType // NEW
}
```

### Шаг 2: Функция `SearchByExactTag()`

Новая функция в том же пакете:

```go
// SearchByExactTag searches for images whose tags exactly match the query.
func SearchByExactTag(db *gorm.DB, query string) ([]SmartSearchResult, error) {
    var tags []domain.ImageTag
    // Exact match, case-sensitive, whitespace-sensitive
    db.Where("tag = ?", query).Find(&tags)

    if len(tags) == 0 {
        return nil, nil
    }

    // Дедупликация по image_file_id
    seen := make(map[uint]bool)
    imageIDs := make([]uint, 0, len(tags))
    for _, t := range tags {
        if !seen[t.ImageFileID] {
            seen[t.ImageFileID] = true
            imageIDs = append(imageIDs, t.ImageFileID)
        }
    }

    // Загрузка ImageFile и тегов (как в SearchByEmbedding)
    ...
}
```

### Шаг 3: Функция `MergeAndRankResults()`

```go
// MergeAndRankResults combines exact-match and embedding search results,
// ranks them, and deduplicates.
func MergeAndRankResults(exactResults, embeddingResults []SmartSearchResult) SmartSearchResponse {
    // Построить map по image_file_id
    // Определить пересечение (both)
    // Собрать три группы: both, exact_only, embedding_only
    // Отсортировать каждую группу
    // Объединить: both → exact_only → embedding_only
}
```

### Шаг 4: Параллельный запуск в handler

В [`handlers_smart_search.go`](../../internal/interfaces/handler/handlers_smart_search.go):

```go
func (s *Server) handleSmartSearch(c *gin.Context) {
    query := c.Query("q")
    // ... валидация ...

    limitStr := c.DefaultQuery("limit", "100")
    limit, err := strconv.Atoi(limitStr)
    if err != nil || limit <= 0 {
        limit = 100
    }
    if limit > 200 {
        limit = 200
    }

    // Параллельный запуск через errgroup
    var exactResults []imaging.SmartSearchResult
    var embeddingResult imaging.SmartSearchResponse

    g, ctx := errgroup.WithContext(c.Request.Context())
    
    g.Go(func() error {
        // SearchByExactTag
    })
    
    g.Go(func() error {
        // SearchByEmbedding с контекстом
    })

    if err := g.Wait(); err != nil {
        // обработка
    }

    // Слияние и ранжирование
    finalResult := imaging.MergeAndRankResults(exactResults, embeddingResult.Images)
    // Обрезать до лимита
    
    c.JSON(http.StatusOK, toDTO(finalResult))
}
```

### Шаг 5: Фронтенд

- Обновить дефолтные лимиты
- Добавить опциональное отображение `matchType` (не обязательно, но полезно для UX)

### Шаг 6: Тесты

- Написать unit-тесты для `SearchByExactTag()`
- Написать unit-тесты для `MergeAndRankResults()`
- Проверить, что существующие тесты проходят

### Шаг 7: API контракт

Обновить в [`api-service.yaml`](../../../docs/api-contracts/api-service.yaml):

```yaml
parameters:
  - name: limit
    in: query
    schema:
      type: integer
      default: 100
```

---

## Потенциальные сложности

1. **Контекст и таймауты**: `SearchByEmbedding` сейчас не принимает context. Нужно передавать контекст для отмены при таймауте или если один из потоков упал.
2. **Производительность**: Exact tag search — это простой индексный lookup (O(1)), проблем не ожидается.
3. **MCP сервер**: `querySemanticSearch()` использует `SearchByEmbedding`. Для AI агента точный тег-матчинг не нужен (агент и так формирует запрос), поэтому MCP можно не менять.
4. **Совместимость API**: Добавление поля `matchType` — обратно совместимо (опциональное поле).

---

## Порядок выполнения

1. Backend: добавить `MatchType`, `SearchByExactTag()`, `MergeAndRankResults()`
2. Backend: изменить handler на параллельный запуск, поднять лимиты
3. Backend: unit-тесты
4. Frontend: обновить лимиты, типы
5. Документация: обновить API контракт
6. Проверка end-to-end
