# Faces dataset — scientists (physicists)

Small, freely licensed photo set of physicists for exercising the `faces`
microservice and the core face-recognition job: face detection, embedding, and
clustering into people.

- 7 photos total: 5 individual portraits and 2 group photos.
- All images are from Wikimedia Commons under a free license (public domain or
  "no known restrictions") and were normalized to at most 1280 px for the
  repository.

## Contents

| File | Type | People | Notes |
|---|---|---|---|
| `einstein_frontal.jpg` | individual | Albert Einstein | Frontal portrait, 1921 |
| `einstein_profile.jpg` | individual | Albert Einstein | Three-quarter profile, 1921 (different angle) |
| `einstein_1947.jpg` | individual | Albert Einstein | Older Einstein, different pose/age |
| `curie_portrait.jpg` | individual | Marie Curie | Profile portrait |
| `bohr_portrait.jpg` | individual | Niels Bohr | Frontal portrait |
| `solvay_1927.jpg` | group (large) | Einstein, Curie, Bohr, Planck, and other Solvay 1927 participants | Fifth Solvay Conference |
| `solvay_1911.jpg` | group (large) | Curie, Planck, Lorentz, Rutherford, and other Solvay 1911 participants | First Solvay Conference |

## Coverage of the intended checks

- **People appearing in more than one photo:** Albert Einstein (4:
  `einstein_frontal.jpg`, `einstein_profile.jpg`, `einstein_1947.jpg`,
  `solvay_1927.jpg`), Marie Curie (3: `curie_portrait.jpg`,
  `solvay_1911.jpg`, `solvay_1927.jpg`), Niels Bohr (2: `bohr_portrait.jpg`,
  `solvay_1927.jpg`).
- **Person in both a group and an individual photo:** Albert Einstein, Marie
  Curie, Niels Bohr.
- **Person shown from different angles:** Albert Einstein — frontal
  (`einstein_frontal.jpg`), three-quarter profile (`einstein_profile.jpg`), and
  a different pose/age in `einstein_1947.jpg`; Marie Curie is a profile portrait
  but frontal in the group photos.
- **Group photos:** `solvay_1927.jpg` and `solvay_1911.jpg` (large groups, faces
  at different scales).

## Sources and licenses

| File | Source (Wikimedia Commons) | Author | License |
|---|---|---|---|
| `einstein_frontal.jpg` | [Albert Einstein 1921 by F Schmutzer (3x4 close cropped)](https://commons.wikimedia.org/wiki/File:Albert_Einstein_1921_by_F_Schmutzer_(3x4_close_cropped).jpg) | Ferdinand Schmutzer | Public domain |
| `einstein_profile.jpg` | [Albert Einstein by Mishkin, 1921](https://commons.wikimedia.org/wiki/File:Albert_Einstein_by_Mishkin,_1921.png) | Herman Mishkin | Public domain |
| `einstein_1947.jpg` | [Albert Einstein 1947](https://commons.wikimedia.org/wiki/File:Albert_Einstein_1947.jpg) | Oren Jack Turner | Public domain |
| `curie_portrait.jpg` | [Marie Sklodowska Curie (1867-1934)](https://commons.wikimedia.org/wiki/File:Marie_Sklodowska_Curie_(1867-1934).jpg) | Smithsonian Institution | No known restrictions (Flickr Commons) |
| `bohr_portrait.jpg` | [Niels Bohr - LOC - ggbain - 35303](https://commons.wikimedia.org/wiki/File:Niels_Bohr_-_LOC_-_ggbain_-_35303.jpg) | Bain News Service; restored by Bammesk | Public domain |
| `solvay_1927.jpg` | [Solvay conference 1927](https://commons.wikimedia.org/wiki/File:Solvay_conference_1927.jpg) | Benjamin Couprie | Public domain |
| `solvay_1911.jpg` | [First solvay conference updated](https://commons.wikimedia.org/wiki/File:First_solvay_conference_updated.png) | Albert K. Johnson | Public domain |

All files are public domain or "no known restrictions"; attribution is recorded
here for completeness.
