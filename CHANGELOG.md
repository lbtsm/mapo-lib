# Changelog

All notable changes to this project are documented in this file.
The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0]

### Added

- `alarm` package: Slack webhook alarm with deduplication, default global
  instance, custom instances, env-var fallback, lazy local IP resolution,
  and a `Sender` interface for mocking.
- `keystore` package: ethereum keystore decryption with both interactive
  (`KeypairFromEth`) and non-interactive (`KeypairFromEthWithPassword`)
  entry points, password caching, and `KEYSTORE_PASSWORD` env support.
- `abi` package: wrapper around go-ethereum's `accounts/abi` with helpers
  for packing inputs, unpacking outputs, decoding calldata, unpacking
  event logs, and looking up methods/events by name or ID. Sentinel
  errors (`ErrMethodNotFound`, `ErrEventNotFound`, `ErrMissingTopics`).
- `contract` package: read-only contract caller. `CallContract` accepts an
  address directly. Legacy `Call`/`CallAt` index-based API kept for
  backwards compatibility and now returns `ErrIndexOutOfRange` instead
  of panicking. `WithFrom` option configures `CallMsg.From`.

### Fixed

- `alarm`: TOCTOU race in dedup check, unbounded `seen` map growth,
  dedup recorded on failure, race on `defaultAlarm`.
- `keystore`: unsynchronised cache map, missing implementation of the
  documented `KEYSTORE_PASSWORD` environment variable.
- `abi`: panic in `UnpackLog` on logs with empty `Topics`.
- `contract`: panic on out-of-range index now returns an error.
