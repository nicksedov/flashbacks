# Faces dataset

Photo sets for exercising the `faces` microservice and the core face-recognition
job: face detection, embedding, and clustering into people. Both sets are
freely licensed.

| Subfolder | Content | Photos |
|---|---|---|
| [`politicians/`](politicians/README.md) | Political figures (Obama, Biden, Merkel, Macron, G7 leaders) | 7 |
| [`scientists/`](scientists/README.md) | Physicists (Einstein, Curie, Bohr; Solvay 1911 and 1927) | 7 |

Each subfolder has its own README with the per-file Wikimedia Commons source,
author and license, plus the coverage of the intended face-recognition checks
(same person in several photos, a person in both a group and an individual
photo, a person from different angles). Images are downscaled to ≤1280 px.

## Automated checks

The photo set is consumed by the pytest integration suite in
[`tests/faces/integration`](../../../tests/faces/integration), which drives a
running `faces` service over HTTP: detection, embedding shape, same-person and
impostor similarity, recognition of a person inside a group photo, and
clustering into people. Run it with `make test-faces-integration`; details and
threshold tuning are in [`tests/faces/README.md`](../../../tests/faces/README.md).
