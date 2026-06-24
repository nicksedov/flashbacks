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

```bash
pip install -r integration/requirements.txt
```

### Запуск

```bash
# По умолчанию: http://ocr:5174
pytest integration/

# С указанием URL (аргумент)
pytest integration/ --base-url http://localhost:5174

# С указанием URL (переменная окружения)
OCR_CLASSIFIER_URL=http://localhost:5174 pytest integration/

# С детализацией
pytest integration/ -v

# Только конкретный тест-класс
pytest integration/ -k "TestHealthEndpoint"

# Только английские документы
pytest integration/ -k "TestClassifyEnglish"
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
