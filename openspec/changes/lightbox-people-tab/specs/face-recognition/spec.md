# Spec Delta — face-recognition

## ADDED Requirements

### Requirement: Detected faces of one image are exposed with their person

`GET /api/image-faces?path=<imagePath>` SHALL return every face detected on that
gallery image, each with its bounding box in the analysed image's pixel frame, its
detection confidence and the person it was clustered into (`personId` is 0 when the
face belongs to none).

<!-- focused-spec:id=face-recognition.image-faces-returned -->
#### Scenario: All detected faces are returned

- **WHEN** an image with N detected faces is requested
- **THEN** the response contains exactly N face entries

<!-- focused-spec:id=face-recognition.image-face-person -->
#### Scenario: A face reports the person it belongs to

- **WHEN** a detected face has been clustered into the person named "Anna"
- **THEN** that face's entry reports the person's id and the name "Anna"

<!-- focused-spec:id=face-recognition.image-face-unassigned -->
#### Scenario: A face without a person reports no person

- **WHEN** a detected face was not clustered into any person
- **THEN** that face's entry reports `personId` 0

<!-- focused-spec:id=face-recognition.image-faces-empty -->
#### Scenario: An image without faces returns an empty list

- **WHEN** an image with no detected faces is requested
- **THEN** the response contains an empty list of faces

### Requirement: Box geometry follows the rendered image

In People mode each bounding box SHALL be scaled together with the displayed image,
so a box keeps covering the same face region at any rendered size.

<!-- focused-spec:id=face-recognition.image-face-box-scaled -->
#### Scenario: A box keeps covering its face after scaling

- **WHEN** an image analysed at 200×100 pixels is rendered at 400×200 pixels and its face box sits at x=50, y=25, width=40, height=20 pixels
- **THEN** the box is rendered at x=100, y=50, width=80, height=40 pixels

### Requirement: The unified lightbox exposes a People mode before Tags

The unified lightbox SHALL offer a People mode, and its tab SHALL render immediately
before the Tags tab.

<!-- focused-spec:id=face-recognition.lightbox-people-tab-order -->
#### Scenario: People tab precedes the Tags tab

- **WHEN** the lightbox tab bar renders
- **THEN** the People tab is the tab immediately before Tags

### Requirement: People mode draws a box for every detected face

In People mode the lightbox SHALL draw a bounding-box overlay on the lightbox image
stage containing one box per detected face.

<!-- focused-spec:id=face-recognition.lightbox-people-box-per-face -->
#### Scenario: Every detected face gets a box

- **WHEN** a user opens the People tab for an image with three detected faces
- **THEN** the image stage renders three bounding boxes

### Requirement: A face belonging to a person is numbered

In People mode each face clustered into a person SHALL carry that person's number,
drawn on the face's bounding box.

<!-- focused-spec:id=face-recognition.lightbox-people-face-numbered -->
#### Scenario: Faces of a person carry the person's number

- **WHEN** the People tab is opened for an image whose two faces belong to two different people
- **THEN** each of the two face boxes is labelled with its person's number

### Requirement: A face without a person is drawn unnumbered

In People mode a face that belongs to no person SHALL be drawn without a number and
visually muted relative to the numbered boxes.

<!-- focused-spec:id=face-recognition.lightbox-people-unassigned-unnumbered -->
#### Scenario: An unassigned face carries no number

- **WHEN** the People tab is opened for an image with one face that belongs to no person
- **THEN** that face's box carries no number

### Requirement: People mode lists the people present in the image

The People panel SHALL list every person present on the image exactly once, each entry
showing the number drawn on that person's bounding box and the person's display name.

<!-- focused-spec:id=face-recognition.lightbox-people-list -->
#### Scenario: Each person appears once with its number

- **WHEN** the People tab is opened for an image where two people each have a face
- **THEN** the panel lists two entries, numbered 1 and 2, matching the numbers on the boxes

### Requirement: An unnamed person falls back to a localized placeholder

A listed person whose name is empty SHALL render the localized placeholder instead of
an empty label.

<!-- focused-spec:id=face-recognition.lightbox-people-name-fallback -->
#### Scenario: An unnamed person shows the placeholder

- **WHEN** the People panel lists a person whose name is empty
- **THEN** that entry renders the localized placeholder that identifies the person

### Requirement: People mode shows an empty state without faces

The People tab SHALL render the shared empty state when the opened image has no
detected faces.

<!-- focused-spec:id=face-recognition.lightbox-people-empty -->
#### Scenario: No faces shows the empty state

- **WHEN** the People tab is opened for an image with no detected faces
- **THEN** the panel shows the empty-state title instead of a people list

### Requirement: People mode strings are localized

Every user-facing string of the People mode SHALL exist in both English and Russian.

<!-- focused-spec:id=face-recognition.lightbox-people-localized -->
#### Scenario: People mode labels are localized

- **WHEN** the interface language is Russian
- **THEN** the People tab and the People panel render Russian labels without falling back to a raw key
