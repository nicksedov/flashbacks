# Общий тестовый датасет

Набор тестовых изображений, общий для всего проекта: он используется не только
OCR-сервисом, но и интеграционными тестами core/webapp, а также ручными
проверками через внешние инструменты (curl, Postman, интеграционные тесты и т.д.).

## Категории

- **eng** — документы на английском языке
- **rus** — документы на русском языке
- **inscription** — короткие фигурные надписи, вывески
- **image** — картинки, фотографии без текстового содержимого

## Где используется

- [`tests/ocr/integration/`](../ocr/integration) — pytest-тесты OCR-сервиса
  (`TestClassifyEnglish`, `TestClassifyRussian`, `TestClassifyImages`,
  `TestClassifyInscriptions`, `TestDatasetFull`);
- [`tests/webapp/integration/`](../webapp/integration) — Vitest HTTP-интеграционные
  тесты core/webapp: изображения-фикстуры и подкаталоги в роли gallery-корней;
- [`backend/ocr/README.md`](../../backend/ocr/README.md) — примеры `curl` с
  изображением из датасета.
