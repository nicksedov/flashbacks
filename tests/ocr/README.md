# Тестовые датасеты

- **eng** — документы на английском языке
- **rus** — документы на русском языке
- **inscription** — короткие фигурные надписи, вывески
- **image** — картинки, фотографии без текстового содержимого

Датасеты могут быть использованы для проверки сервиса с использованием
внешних инструментов (curl, Postman, интеграционные тесты и т.д.).

---

## Интеграционные тесты (Python)

Python-тесты в `integration/` тестируют **развёрнутый** сервис через HTTP.
Они покрывают те же сценарии, что и удалённые Go-юнит-тесты.

### Требования

Тесты запускаются в отдельном виртуальном окружении Python:

```bash
# Создать venv и установить зависимости (то же самое: make setup-pytest)
python3 -m venv integration/.venv
integration/.venv/bin/pip install -r integration/requirements.txt
```

### Запуск

Сервис OCR должен быть запущен локально (см. `make run-ocr` или
`make dev-ocr`). По умолчанию тесты обращаются к `http://localhost:5174`.

```bash
# Через Makefile: при необходимости поднимет OCR и запустит все тесты
make test-ocr-integration

# Напрямую через venv (из каталога tests/ocr)
integration/.venv/bin/python -m pytest integration/

# С указанием URL (аргумент)
integration/.venv/bin/python -m pytest integration/ --base-url http://localhost:5174

# С указанием URL (переменная окружения)
OCR_CLASSIFIER_URL=http://localhost:5174 integration/.venv/bin/python -m pytest integration/

# Только конкретный тест-класс
integration/.venv/bin/python -m pytest integration/ -k "TestHealthEndpoint"

# Только английские документы
integration/.venv/bin/python -m pytest integration/ -k "TestClassifyEnglish"
```

### Что тестируется

| Тест | Описание |
|------|----------|
| `TestHealthEndpoint` | Health-check эндпоинт |
| `TestClassifyErrors` | Обработка ошибок (метод, Content-Type, пустое тело) |
| `TestClassifyEnglish` | Все документы из `dataset/eng/` — должны быть `is_text_document=true` |
| `TestClassifyRussian` | Все документы из `dataset/rus/` — должны быть `is_text_document=true` |
| `TestClassifyRussianRotated` | Повёрнутые русские документы — коррекция угла |
| `TestClassifyImages` | Изображения без текста (`dataset/image/`) |
| `TestClassifyInscriptions` | Короткие надписи (`dataset/inscription/`) |
| `TestClassifyParameters` | Query-параметры (`lang`, `level`, `confidence_threshold`, `min_token_count`) |
| `TestDatasetFull` | Пакетная проверка всех изображений во всех категориях |
