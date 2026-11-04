# Spec Delta

## Purpose

Defines the operational contract for backend microservices: every service is
buildable, wired into local development, and kept in sync by the SDD workflow
whenever a new microservice is introduced, including the deploy-skill assets
that the workflow enumerates.

## ADDED Requirements

### Requirement: The SDD workflow mandates deployment-asset updates for new microservices

The SDD workflow SHALL require that any change introducing a backend microservice update the staging deploy skill, the production deploy skill, the local dev tooling and the shared service documentation, and SHALL enumerate the concrete files to update.

<!-- focused-spec:id=service-deployment.workflow-deploy-step -->
#### Scenario: Workflow contains the deploy-assets step

- **WHEN** an agent reads the SDD workflow document
- **THEN** it finds a dedicated new-microservice deployment-assets step that names the staging and production deploy skill files to update

<!-- focused-spec:id=service-deployment.workflow-definition-of-done -->
#### Scenario: Workflow definition of done includes deployment assets

- **WHEN** an agent checks the SDD workflow definition of done
- **THEN** it contains a criterion that deployment assets were updated for every new or changed microservice

### Requirement: Local development tooling starts every backend microservice

The host-native local dev tooling SHALL manage every backend microservice, so the dev scripts start it with the correct port and health path and the shared environment example documents its peer URL.

<!-- focused-spec:id=service-deployment.local-dev-manages-faces -->
#### Scenario: Dev stack manages the faces service

- **WHEN** a developer starts the host-native dev stack
- **THEN** the faces service is one of the managed services with its port and health path, and the shared environment example defines its peer URL

<!-- focused-spec:id=service-deployment.make-targets-for-faces -->
#### Scenario: Make targets exist for the faces service

- **WHEN** a developer builds, tests or starts the faces service through the Makefile
- **THEN** dedicated make targets for building, testing, running and setting up faces exist

### Requirement: The faces image reports real readiness

The faces container image SHALL run as a non-root user and expose a health probe over its HTTP service, so an orchestrator can gate dependent services on it.

<!-- focused-spec:id=service-deployment.faces-image-healthcheck -->
#### Scenario: Faces image healthcheck and non-root runtime

- **WHEN** the faces image definition is inspected
- **THEN** it declares a healthcheck that probes `/faces/health` and runs the application as a non-root user
