# service-registry Specification

## Purpose

Defines how the Flashbacks services obtain configuration and peer locations from a central registry, including live configuration updates, TTL-based service discovery, bootstrap fallback, and encrypted secret storage.

## Requirements

### Requirement: Central configuration store

The registry SHALL expose an HTTP API that stores, lists, and returns named configuration values per service, so every service reads application configuration from one source of truth instead of local environment files.

<!-- focused-spec:id=service-registry.config-roundtrip -->
#### Scenario: Configuration value round-trip

- **WHEN** a configuration value is written for service `exif` and key `db_host`
- **THEN** a read of that service and key returns the stored value

### Requirement: Configuration watch notifications

The registry SHALL notify subscribed services when a configuration value under their service prefix changes, so a running service can reload configuration without restarting.

<!-- focused-spec:id=service-registry.config-watch -->
#### Scenario: Update notifies the watcher

- **WHEN** a service watches its configuration prefix and another actor updates a key under that prefix
- **THEN** the registry delivers the updated key and value to the watcher

### Requirement: Service self-registration with a lease

Each service instance SHALL register itself under a time-to-live lease and refresh that lease while it runs, so discovery only returns live instances.

<!-- focused-spec:id=service-registry.register-discovery -->
#### Scenario: Registration appears in discovery

- **WHEN** a service instance registers itself with a TTL lease
- **THEN** discovery for that service name returns the instance address

### Requirement: Expired registration removal

The registry SHALL remove an instance from discovery when its lease expires, so callers never route traffic to an instance that stopped refreshing its lease.

<!-- focused-spec:id=service-registry.lease-expiry -->
#### Scenario: Expired lease disappears from discovery

- **WHEN** a registered instance stops refreshing its lease for longer than its TTL
- **THEN** discovery for that service name no longer returns the instance

### Requirement: Discovery-based peer resolution

A consumer service SHALL resolve a peer's base URL from the registry's discovery data rather than from a static environment variable, so peer location changes propagate without redeploying the consumer.

<!-- focused-spec:id=service-registry.discovery-resolves-peer -->
#### Scenario: Core resolves the exif base URL

- **WHEN** `core` requests the registered instances of `exif`
- **THEN** it receives the registered base URL used to build EXIF requests

### Requirement: Bootstrap fallback on registry outage

A service SHALL continue startup with a configured bootstrap peer URL when the registry is unreachable, so a registry outage does not prevent the service from running.

<!-- focused-spec:id=service-registry.bootstrap-fallback -->
#### Scenario: Unavailable registry uses the bootstrap URL

- **WHEN** the registry is unreachable at startup and a bootstrap peer URL is configured
- **THEN** the service uses the bootstrap URL for that peer

### Requirement: Bootstrap-only environment configuration

A service SHALL require only the registry endpoint, credentials, and its own identity from environment variables, and SHALL read every other configuration value from the registry.

<!-- focused-spec:id=service-registry.bootstrap-only-env -->
#### Scenario: Service starts with bootstrap-only environment

- **WHEN** a service starts with only the registry endpoint, credentials, and identity environment variables set
- **THEN** it loads its application configuration from the registry

### Requirement: Secret encryption at rest

The registry SHALL persist secret values encrypted with AES-256-GCM and SHALL never store the submitted plaintext, so secret material is unreadable in the underlying key-value store.

<!-- focused-spec:id=service-registry.secret-encrypted-at-rest -->
#### Scenario: Stored secret is ciphertext

- **WHEN** a secret value is written for a service
- **THEN** the value persisted in the registry key-value backend differs from the submitted plaintext

### Requirement: Secret read round-trip

The registry SHALL return the original plaintext for a stored secret through its secrets read API, so consuming services receive usable credential values.

<!-- focused-spec:id=service-registry.secret-roundtrip -->
#### Scenario: Secret read returns plaintext

- **WHEN** a secret value is written and then read through the secrets API
- **THEN** the read returns the original plaintext value

### Requirement: Registry health readiness

The registry SHALL expose a health endpoint that reports readiness of its embedded key-value backend, so dev tooling and orchestrators can gate dependent service startup.

<!-- focused-spec:id=service-registry.health-ready -->
#### Scenario: Health reports ready

- **WHEN** the registry is running with its embedded key-value backend initialized
- **THEN** its health endpoint returns a success status

### Requirement: Configuration precedence

When resolving configuration values, a service SHALL apply explicit environment overrides over registry values, and registry values over built-in defaults, so a local debugging override does not require changing the shared registry.

<!-- focused-spec:id=service-registry.config-precedence -->
#### Scenario: Environment override wins over the registry value

- **WHEN** a configuration key has a built-in default, a registry value, and an explicit environment override
- **THEN** the resolved value is the environment override

<!-- focused-spec:id=service-registry.config-registry-over-default -->
#### Scenario: Registry value wins over the built-in default

- **WHEN** a configuration key has a built-in default and a registry value but no environment override
- **THEN** the resolved value is the registry value

### Requirement: Configuration fallback when the registry is unreachable

A service SHALL resolve a configuration key from its environment override or built-in default when the registry cannot serve the stored values, so a registry outage degrades configuration instead of blocking startup.

<!-- focused-spec:id=service-registry.config-fallback -->
#### Scenario: Unavailable registry falls back to the built-in default

- **WHEN** the registry is unreachable while a service resolves a configuration key that has a built-in default
- **THEN** the resolved value is the built-in default instead of a startup failure

### Requirement: Instance-change watch notifications

The registry SHALL notify a subscribed consumer when an instance of a service registers or deregisters, so peer discovery updates without the consumer restarting.

<!-- focused-spec:id=service-registry.instance-watch -->
#### Scenario: Registration notifies an instance watcher

- **WHEN** a consumer watches a service's instance registrations and another instance registers itself
- **THEN** the registry delivers that registration event to the watcher
