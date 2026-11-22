# Spec Delta

## ADDED Requirements

### Requirement: The People detail follows a refreshed catalog

After the People tab refreshes its catalog, a person that is no longer in the
fresh list SHALL NOT remain open: the tab SHALL return to the People grid and
SHALL NOT render that person's name or the photos of the person's former
cluster.

<!-- focused-spec:id=face-recognition.people-detail-closes-when-person-gone -->
#### Scenario: The opened person disappears from the catalog

- **WHEN** the person whose detail is open is absent from the catalog the tab just loaded
- **THEN** the tab shows the People grid with only the people of that catalog

### Requirement: The People tab reloads its catalog when it becomes visible

The People tab SHALL reload the people catalog every time it becomes the visible
view, so a cluster merged in another view, tab or browser session SHALL NOT stay
listed.

<!-- focused-spec:id=face-recognition.people-list-refreshes-on-visibility -->
#### Scenario: A cluster was merged while the tab was hidden

- **WHEN** the People tab becomes visible again after another view folded a cluster into a surviving person
- **THEN** the grid lists the refreshed catalog without the folded-away cluster

### Requirement: Catalog requests are never answered from the webapp cache

The webapp service worker SHALL NOT read from or write to its cache for `/api/`
requests. A catalog listing SHALL either come from the live backend or fail; a
payload cached by an earlier build SHALL NOT be replayed as the current catalog.

<!-- focused-spec:id=face-recognition.catalog-request-network-only -->
#### Scenario: A cached people payload is not replayed

- **WHEN** the webapp requests a `/api/` endpoint while the backend is unreachable
- **THEN** the service worker returns a network failure instead of a previously cached response
