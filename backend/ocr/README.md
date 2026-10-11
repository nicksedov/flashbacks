# OCR

HTTP-сервис для определения наличия текста на изображениях с использованием Tesseract OCR. Возвращает детальную информацию о найденном тексте, включая показатели уверенности (confidence), координаты текстовых блоков и количество найденных токенов. Ответ содержит вердикт, является ли данное изображение документом (true/false), который выносится на основе правил.

## Требования

- Go 1.25+
- Tesseract OCR (libtesseract + leptonica) и языковые данные eng/rus

### Установка Tesseract

Сервис линкуется с libtesseract/libleptonica через CGO (gosseract), поэтому ему
нужны и заголовочные файлы, и разделяемые библиотеки. В этом проекте Tesseract
устанавливается **локально для проекта** в `.tools/tesseract/` — без root и без
системного `apt` (тот же подход, что и для ExifTool):

```bash
make tools-tesseract   # только Tesseract
make tools             # ExifTool + Tesseract
```

`scripts/bootstrap.sh tesseract` резолвит актуальные `.deb`-пакеты Ubuntu
(`tesseract-ocr`, `libtesseract5`, `libtesseract-dev`, `liblept5`,
`libleptonica-dev`, языковые данные eng/rus/osd и недостающие
runtime-зависимости) и распаковывает их в `.tools/tesseract/`. Скрипт
`scripts/toolenv.sh` экспортирует `PATH`, `LD_LIBRARY_PATH`, `CGO_CPPFLAGS`,
`CGO_CXXFLAGS`, `CGO_LDFLAGS` и `TESSDATA_PREFIX` — их автоматически
подхватывают `make` и `scripts/dev.sh`.

Альтернативно можно использовать системный Tesseract (с обязательными
заголовками разработки):

**Windows:**
```bash
# Установить через Chocolatey
choco install tesseract
```

**Linux (Ubuntu/Debian):**
```bash
sudo apt-get install tesseract-ocr tesseract-ocr-rus libtesseract-dev
```

**macOS:**
```bash
brew install tesseract
```

### Run Locally

```bash
# Build the binary
go build -o ocr ./cmd/server

# Run the service
./ocr
```

### Run with Docker

```bash
# Build the image
docker build -t ocr .

# Run the container
docker run -p 5174:5174 ocr
```

## API

Все эндпоинты находятся под корневым путём `/ocr/api`.

### Health Check

Проверка работоспособности сервиса.

```
GET /ocr/api/health
```

**Ответ:**
```json
{"status": "ok"}
```

### Classify (v1)

Классификация изображения на наличие текста. Поддерживаются форматы `image/jpeg` и `image/png`.

```
POST /ocr/api/v1/classify
Content-Type: image/jpeg
Body: <бинарные данные изображения>
```

**Query параметры:**

- `lang` — языки для Tesseract OCR (например, `eng`, `rus`, `eng+rus`). По умолчанию: `eng+rus`
- `level` — уровень детализации структуры текста (PageIteratorLevel): `RIL_BLOCK`, `RIL_PARA`, `RIL_TEXTLINE`, `RIL_WORD`, `RIL_SYMBOL`. По умолчанию: `RIL_WORD`
- `confidence_threshold` — минимальный порог уверенности (0-1). По умолчанию: 0.66
- `min_token_count` — минимальное количество токенов. По умолчанию: 20

**Успешный ответ (200):**
```json
{
  "mean_confidence": 0.85,
  "weighted_confidence": 0.88,
  "token_count": 12,
  "boxes": [
    {
      "x": 10,
      "y": 20,
      "width": 100,
      "height": 50,
      "word": "Example",
      "confidence": 0.95
    }
  ],
  "angle": 0,
  "scale_factor": 1.0,
  "is_text_document": false,
  "bounding_box_width": 800,
  "bounding_box_height": 600
}
```

**Ошибки (4xx/5xx):**
```json
{"error": "сообщение об ошибке"}
```

- `405` - неверный HTTP метод (только POST)
- `400` - неверный Content-Type, пустое изображение или ошибка чтения данных
- `500` - ошибка обработки изображения или Tesseract OCR

## Тестирование с помощью curl

### Health Check

```bash
curl http://localhost:5174/ocr/api/health
```

### Классификация изображения

```bash
curl -X POST \
  -H "Content-Type: image/jpeg" \
  --data-binary @path/to/image.jpg \
  http://localhost:5174/ocr/api/v1/classify
```

**Пример с параметрами:**

```bash
curl -X POST \
  -H "Content-Type: image/jpeg" \
  --data-binary @test/dataset/eng/lightbulb-scheme.jpg \
  "http://localhost:5174/ocr/api/v1/classify?lang=eng&level=RIL_WORD&confidence_threshold=0.7&min_token_count=10"
```

**Пример c изображением из датасета:**

```bash
curl -X POST \
  -H "Content-Type: image/jpeg" \
  --data-binary @test/dataset/eng/lightbulb-scheme.jpg \
  http://localhost:5174/ocr/api/v1/classify
```
