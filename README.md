# Aitvaras

Aitvaras is an open-source control plane and SDK for coordinating a fleet of independently operated execution nodes.

Nodes expose narrow capabilities such as policy evaluation or signing. Control discovers and routes to those capabilities, tracks operation state, and records evidence. Every Node enforces its own configuration; Control cannot widen a Node's local policy.

Application clients are outside this repository's core scope. A trading bot, for example, uses an Aitvaras SDK but remains responsible for strategy and venue-specific logic. The `examples/` directory contains protocol requests and local configuration only. Complete applications should live in separate repositories.

## Repository scope

- **Control plane:** node registration, capability discovery, routing, operation state and evidence.
- **SDKs:** typed client and Node APIs derived from versioned protocol contracts.
- **Node runtime:** common verification, policy, replay and receipt behavior for a fleet of independently operated Nodes.
- **Node capabilities:** Policy Node and Signer Node first; additional capabilities require separate specifications.

Hosted and self-hosted deployments use the same protocol. Deployment location does not alter validation or evidence semantics.

## Current implementation

The first local slice includes an HTTP Control service and a keyless Budget Policy Node. Control forwards requests to one configured Node. The Node atomically records separate spend and gas reservations in an append-only local journal.

An accepted response has status `reserved_simulation`. It is accounting evidence only. The current code does not create wallets, sign, broadcast or move funds. Earlier in-memory registration and typed-intent handlers remain compatibility scaffolding outside the budget path.

## Run the local slice

Requires Go 1.26 or newer on macOS or Linux. The demonstration uses three separate local tokens.

Initialize a journal once:

```sh
go run ./cmd/policy-node -init
```

Start the Policy Node:

```sh
RING29_NODE_TOKEN=demo-node-only go run ./cmd/policy-node
```

Start Control in another terminal:

```sh
export RING29_DEV_TOKEN=demo-admin-only
export RING29_CLIENT_TOKEN=demo-client-only
export RING29_NODE_TOKEN=demo-node-only
export RING29_BUDGET_NODE_URL=http://127.0.0.1:8082
go run ./cmd/core-api
```

Submit the protocol example:

```sh
curl -sS http://127.0.0.1:8080/v1/budget/reservations \
  -H 'Authorization: Bearer demo-client-only' \
  -H 'Content-Type: application/json' \
  --data-binary @examples/budget-reservation.json
```

Retrying the same request ID with identical fields returns the stored receipt. Reusing the ID with changed fields returns `409`. Query current accounting with:

```sh
curl -sS http://127.0.0.1:8080/v1/budget \
  -H 'Authorization: Bearer demo-client-only'
```

The example uses integer base units. Gas reservation is `gas_limit × max_fee_per_gas`. The implementation does not validate chain balances, decode transaction calldata, model network-specific additional fees, release reservations or update policy.

## Verification

```sh
go test -race ./...
go vet ./...
```

Tests cover authentication boundaries, strict JSON, scope denial, separate budget exhaustion, concurrent accounting, idempotency, conflicting request IDs, restart replay and storage failures. These are local test results, not staging or production evidence.

## Documentation

- [Project scope](INTENT.md)
- [Architecture](docs/architecture/aitvaras.md)
- [Budget Policy Node v0](docs/architecture/budget-node-v0.md)
- [RIP-0001](docs/rfcs/0001-aitvaras.md)
- [Budget API](api/budget.openapi.yaml)
- [Compatibility API](api/openapi.yaml)

## License and provenance

Apache-2.0. See [ORIGIN.md](ORIGIN.md) for the clean-room contribution boundary.
