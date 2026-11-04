# Faces — интеграционные тесты распознавания лиц

`integration/` — pytest-набор, который проверяет **развёрнутый** сервис `faces`
через HTTP (`/faces/health`, `/faces/detect`, `/faces/embed`, `/faces/cluster`)
на общем датасете фотографий [`tests/dataset/faces`](../dataset/faces/README.md).

Сам набор не запускает ML-модели: детекция и эмбеддинги выполняются сервисом,
pytest только сравнивает полученные векторы. Поэтому ни `insightface`, ни
`onnxruntime` в тестовом окружении не нужны.

---

## Требования

Сервис `faces` должен быть запущен и уже скачать модели InsightFace
(`~/.insightface`):

```bash
make setup-faces     # один раз: venv сервиса + зависимости (включая onnxruntime/insightface)
make run-faces       # запуск сервиса в фоне (порт 5173)
```

Виртуальное окружение для тестов создаётся автоматически:

```bash
make setup-faces-pytest   # python3 -m venv tests/faces/integration/.venv + requirements.txt
```

## Запуск

```bash
# Через Makefile: при необходимости создаст venv и поднимет faces
make test-faces-integration

# Напрямую через venv (из корня репозитория)
tests/faces/integration/.venv/bin/python -m pytest tests/faces/integration

# Указать URL сервиса
tests/faces/integration/.venv/bin/python -m pytest tests/faces/integration \
  --base-url http://localhost:5173

# Через переменную окружения
FACES_SERVICE_URL=http://localhost:5173 \
  tests/faces/integration/.venv/bin/python -m pytest tests/faces/integration

# Только распознавание лиц (один человек / разные люди / групповые снимки)
tests/faces/integration/.venv/bin/python -m pytest tests/faces/integration -k "same_person or different_people or group"

# Только кластеризация
tests/faces/integration/.venv/bin/python -m pytest tests/faces/integration -k cluster
```

Если сервис недоступен, сессия прерывается с подсказкой `make run-faces`.
Учтите: первый запрос после старта сервиса лениво загружает модели
(`/faces/detect` может занять десятки секунд), поэтому таймаут запроса по
умолчанию — 120 с.

## Что тестируется

| Модуль | Проверки |
|--------|----------|
| `test_service.py` | `/faces/health`; POST-only детекция; отсутствие поля `image` (422); невалидные base64/изображения (400); пустой набор эмбеддингов → пустые метки |
| `test_detect.py` | Каждый портрет → есть уверенное лицо; групповые снимки → минимум N лиц; геометрия боксов (положительный размер, внутри кадра); `dataset/image/light-bulb.jpg` → лиц нет; полный обход датасета |
| `test_embed.py` | Размерность 512, конечные значения, L2-норма ≈ 1; детерминизм; эмбеддинг кропа ≈ эмбеддинг полного кадра; разные люди → разные векторы |
| `test_recognition.py` | Один человек на разных снимках → косинус выше порога; разные люди → ниже порога импостора; человек из группового снимка находится по портретам (галерея); `obama_biden.jpg` совпадает ровно с одним человеком |
| `test_cluster.py` | Кластеры не смешивают разных людей; один человек не разбивается на несколько кластеров; хотя бы одна персона с несколькими снимками образует собственный кластер |

### Датасет и «эталонные» ожидания

Соответствие «человек → фотографии» и список групповых снимков зашиты в
[`integration/support.py`](integration/support.py) (`IDENTITY_PORTRAITS`,
`GROUP_PHOTOS`, `GROUP_IDENTITY_PAIRS`, `SAME_PERSON_PAIRS`, `IMPOSTOR_PAIRS`)
и повторяют таблицы из [`dataset/faces/README.md`](../dataset/faces/README.md).
При добавлении фотографий в датасет обновите эти таблицы — обход датасета
(`test_every_dataset_photo_returns_a_valid_response`) подхватит новые файлы
автоматически.

### Пороги и их настройка

Пороги подобраны консервативно и вынесены в CLI/окружение, чтобы калибровать
набор под конкретную модель и железо, не меняя код:

| CLI | Переменная окружения | По умолчанию | Смысл |
|-----|----------------------|--------------|-------|
| `--base-url` | `FACES_SERVICE_URL` | `http://localhost:5173` | Адрес сервиса (без префикса `/faces`) |
| `--faces-request-timeout` | `FACES_REQUEST_TIMEOUT` | `120` | Таймаут одного HTTP-запроса, с |
| `--min-detection-confidence` | `FACES_MIN_DETECTION_CONFIDENCE` | `0.5` | Минимальная уверенность, чтобы лицо считалось найденным |
| `--same-person-cosine` | `FACES_SAME_PERSON_COSINE` | `0.40` | Верхняя граница для пар «один человек»: может ослабить пары, но не ужесточить |
| `--impostor-cosine` | `FACES_IMPOSTOR_COSINE` | `0.35` | Максимальный косинус для разных людей |
| `--group-cosine` | `FACES_GROUP_COSINE` | `0.30` | Минимальный косинус при поиске человека в групповом снимке |
| `--cluster-min-size` | `FACES_CLUSTER_MIN_SIZE` | `2` | `minClusterSize` для `/faces/cluster` |

Пороги для отдельных пар (например, Эйнштейн 1921 vs 1947 — разница в возрасте
26 лет) заданы в `SAME_PERSON_PAIRS` рядом с самими парами. То же для пар
«групповой снимок → человек»: в `GROUP_IDENTITY_PAIRS` третьим элементом идёт
свой порог. Так, у Меркель на `g7_family_photo.jpg` лицо всего ~20×28 px
(снимок уменьшен до 1280 px), и её вектор даёт ~0.22 при ~0.10 у ближайшего
чужого лица — порог для этой пары ослаблен до `0.20`.

## Замечания

- Набор сознательно последовательный (без `pytest-xdist`): эмбеддинги
  кэшируются на процесс, поэтому каждый снимок отправляется в сервис один раз
  за прогон. Полный прогон — порядка 60 инференсов на CPU.
- Кластеризация — DBSCAN по косинусному расстоянию с порогом сходства `0.40`
  (см. `CLUSTER_MIN_SIMILARITY` в [`engine.py`](../../backend/faces/app/engine.py)):
  одиночный снимок остаётся шумом (`-1`) — это допустимо; недопустимо только
  смешивание людей в одном кластере и разбиение одного человека на несколько
  кластеров.
