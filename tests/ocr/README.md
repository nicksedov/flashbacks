# OCR — интеграционные тесты

Python-тесты в `integration/` тестируют **развёрнутый** сервис через HTTP.
Они покрывают те же сценарии, что и удалённые Go-юнит-тесты.

Тестовые изображения вынесены в общий датасет
[`tests/dataset`](../dataset/README.md) — он используется не только OCR.

---

## Требования

Тесты запускаются в отдельном виртуальном окружении Python:

```bash
# Создать venv и установить зависимости (то же самое: make setup-pytest)
python3 -m venv integration/.venv
integration/.venv/bin/pip install -r integration/requirements.txt
```

## Запуск

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
OCR_SERVICE_URL=http://localhost:5174 integration/.venv/bin/python -m pytest integration/

# Только конкретный тест-класс
integration/.venv/bin/python -m pytest integration/ -k "TestHealthEndpoint"

# Только английские документы
integration/.venv/bin/python -m pytest integration/ -k "TestClassifyEnglish"
```

## Что тестируется

| Тест | Описание |
|------|----------|
| `TestHealthEndpoint` | Health-check эндпоинт |
| `TestClassifyErrors` | Обработка ошибок (метод, Content-Type, пустое тело) |
| `TestClassifyEnglish` | Все документы из `tests/dataset/eng/` — должны быть `is_text_document=true` |
| `TestClassifyRussian` | Все документы из `tests/dataset/rus/` — должны быть `is_text_document=true` |
| `TestClassifyRussianRotated` | Повёрнутые русские документы — коррекция угла |
| `TestClassifyImages` | Изображения без текста (`tests/dataset/image/`) |
| `TestClassifyInscriptions` | Короткие надписи (`tests/dataset/inscription/`) |
| `TestClassifyParameters` | Query-параметры (`lang`, `level`, `confidence_threshold`, `min_token_count`) |
| `TestDatasetFull` | Пакетная проверка всех изображений во всех категориях |
