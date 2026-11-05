# Общий тестовый датасет

Набор тестовых изображений, общий для всего проекта: он используется не только
OCR-сервисом, но и интеграционными тестами core/webapp, а также ручными
проверками через внешние инструменты (curl, Postman, интеграционные тесты и т.д.).

## Категории

- **eng** — документы на английском языке
- **rus** — документы на русском языке
- **inscription** — короткие фигурные надписи, вывески
- **image** — картинки, фотографии без текстового содержимого
- **faces** — фотографии людей для проверки распознавания лиц:
  `politicians/` и `scientists/` (свободные лицензии) и `dima/` — приватный
  размеченный набор из production-галереи (один человек, разбитый
  кластеризацией на шесть «людей»), с `manifest.csv` и координатами лиц

## Где используется

- [`tests/ocr/integration/`](../ocr/integration) — pytest-тесты OCR-сервиса
  (`TestClassifyEnglish`, `TestClassifyRussian`, `TestClassifyImages`,
  `TestClassifyInscriptions`, `TestDatasetFull`);
- [`faces/`](faces) — фотографии людей для проверок распознавания лиц: подпапки
  [`faces/politicians/`](faces/politicians), [`faces/scientists/`](faces/scientists)
  и [`faces/dima/`](faces/dima) — размеченный набор для регрессии кластеризации
  (см. [`faces/README.md`](faces/README.md) и [`faces/dima/README.md`](faces/dima/README.md));
- [`tests/faces/integration/`](../faces/integration) — pytest-тесты распознавания
  лиц против развёрнутого сервиса `faces` (детекция, эмбеддинги, распознавание
  в групповых снимках, кластеризация по людям; см.
  [`tests/faces/README.md`](../faces/README.md));
- [`tests/webapp/integration/`](../webapp/integration) — Vitest HTTP-интеграционные
  тесты core/webapp: изображения-фикстуры и подкаталоги в роли gallery-корней;
- [`backend/ocr/README.md`](../../backend/ocr/README.md) — примеры `curl` с
  изображением из датасета.
