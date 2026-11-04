# Faces dataset — politicians

Small, freely licensed photo set of political figures for exercising the `faces`
microservice and the core face-recognition job: face detection, embedding, and
clustering into people.

- 7 photos total: 4 individual portraits and 3 group photos.
- All images are from Wikimedia Commons under a free license (Public domain,
  CC BY 4.0, or CC BY-SA 4.0) and were downscaled to at most 1280 px for the
  repository.

## Contents

| File | Type | People | Notes |
|---|---|---|---|
| `obama_front.jpg` | individual | Barack Obama | Frontal official portrait |
| `obama_profile.jpg` | individual | Barack Obama | Different angle: looking up / three-quarter profile |
| `obama_biden.jpg` | group (2) | Barack Obama, Joe Biden | Two-person group |
| `obama_cabinet.jpg` | group (large) | Barack Obama + cabinet (incl. Joe Biden) | Large group, faces at different scales |
| `merkel_portrait.jpg` | individual | Angela Merkel | Frontal portrait |
| `macron_portrait.jpg` | individual | Emmanuel Macron | Frontal portrait |
| `g7_family_photo.jpg` | group (large) | G7 leaders (incl. Angela Merkel, Emmanuel Macron, Donald Trump, Justin Trudeau, Boris Johnson) | Large group, outdoor lighting |

## Coverage of the intended checks

- **People appearing in more than one photo:** Barack Obama (4 photos), Joe Biden
  (2: `obama_biden.jpg`, `obama_cabinet.jpg`), Angela Merkel (2:
  `merkel_portrait.jpg`, `g7_family_photo.jpg`), Emmanuel Macron (2:
  `macron_portrait.jpg`, `g7_family_photo.jpg`).
- **Person in both a group and an individual photo:** Barack Obama, Angela
  Merkel, Emmanuel Macron.
- **Person shown from different angles:** Barack Obama — frontal
  (`obama_front.jpg`) vs. three-quarter/profile (`obama_profile.jpg`).
- **Group photos:** `obama_biden.jpg` (2 people), `obama_cabinet.jpg` (large),
  `g7_family_photo.jpg` (large).

## Sources and licenses

| File | Source (Wikimedia Commons) | Author | License |
|---|---|---|---|
| `obama_front.jpg` | [President Barack Obama](https://commons.wikimedia.org/wiki/File:President_Barack_Obama.jpg) | Official White House Photo by Pete Souza | Public domain |
| `obama_profile.jpg` | [Barack Obama profile picture](https://commons.wikimedia.org/wiki/File:Barack_Obama_profile_picture.jpg) | Lawrence Jackson (White House) | Public domain |
| `obama_biden.jpg` | [Barack Obama and Joe Biden in the Oval Office - 2015](https://commons.wikimedia.org/wiki/File:Barack_Obama_and_Joe_Biden_in_the_Oval_Office_-_2015.jpg) | Pete Souza (White House) | Public domain |
| `obama_cabinet.jpg` | [President Barack Obama with full cabinet 09-10-09](https://commons.wikimedia.org/wiki/File:President_Barack_Obama_with_full_cabinet_09-10-09.jpg) | Chuck Kennedy (White House) | Public domain |
| `merkel_portrait.jpg` | [Angela Merkel 2019 cropped](https://commons.wikimedia.org/wiki/File:Angela_Merkel_2019_cropped.jpg) | Raimond Spekking | CC BY-SA 4.0 |
| `macron_portrait.jpg` | [Emmanuel Macron June 2022 (cropped)](https://commons.wikimedia.org/wiki/File:Emmanuel_Macron_June_2022_(cropped).jpg) | President of Ukraine official website | CC BY 4.0 |
| `g7_family_photo.jpg` | [-G7Biarritz (48632300193)](https://commons.wikimedia.org/wiki/File:-G7Biarritz_(48632300193).jpg) | The White House | Public domain |

Attribution notice: the CC BY 4.0 and CC BY-SA 4.0 files above are used with the
attribution recorded in this table; the CC BY-SA 4.0 file remains under the same
license. All other files are public domain.
