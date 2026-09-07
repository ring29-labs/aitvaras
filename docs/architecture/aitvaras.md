# Aitvaras architecture

- Accepted: Control plane, SDK boundary, independently operated node fleet, capability-based roles.
- Implemented locally: HTTP Control scaffold and one keyless Budget Policy Node.
- Proposed: fleet discovery and routing, versioned Node Protocol, public SDK, signing execution owner and durable evidence.
- Deferred: production identity, production key providers, MPC, multi-host fencing and mainnet operation.

## System boundary

Aitvaras coordinates operations without making Control sufficient to perform them. Clients call Control through an SDK. Control selects a compatible Node. The selected Node authenticates the request, reconstructs its meaning, applies local policy and produces evidence for the state it actually reached.

```text
External clients
      │
      ▼
Client SDK
      │
      ▼
Control plane
      │
      ├──────── registry / routing / operation state
      │
      ▼
Node SDK and protocol
      │
      ▼
Fleet of independently operated Nodes
      ├── policy capability
      ├── signer capability
      └── combined capability sets
```

An application client is not a Node. Client business logic and integrations remain outside the core repository. The SDK is the supported boundary between those applications and Aitvaras.

## Control plane

Control is responsible for:

- authenticating client and Node principals;
- registering Node identity, epoch and declared capabilities;
- discovering eligible Nodes by scope, capability and readiness;
- routing an operation to its execution owner;
- tracking deadlines and independent state dimensions;
- serving durable receipts and evidence to authorized callers.

Registration is not attestation. Health is not authorization. A routing decision does not override Node-local policy. Control must not accept a caller-provided Node URL or RPC endpoint as an execution destination.

## Node fleet

A deployment may contain many Nodes with different operators, locations and capabilities. A Node descriptor is expected to bind:

- stable Node identity and enrollment epoch;
- operator and trust profile;
- protocol and schema versions;
- declared capability names and versions;
- authorized organizations, vaults, wallets and networks;
- authenticated transport endpoint;
- readiness and quarantine state.

Roles are capabilities, not fixed process types. One runtime may expose policy only, signing plus local policy, or another explicitly specified combination. A Signer Node always retains a local policy floor even when Control or another Policy Node already evaluated the request.

Fleet size does not remove ownership constraints. Each `(chain_id, from_address)` execution lane has one active owner for nonce, replay, budget and durable result state. A queue group must not distribute adjacent nonces across arbitrary signers. Failover requires fencing the previous owner from its Key Provider; a lease expiry alone is insufficient.

## SDK boundary

Versioned protocol schemas are the source for SDK types. The initial SDK should provide:

- a client for capability discovery, operation submission and receipt retrieval;
- Node handlers for request verification and typed responses;
- stable identifiers, error codes and idempotency behavior;
- transport adapters that preserve the same domain contract;
- conformance fixtures shared by hosted and self-hosted deployments.

SDKs must not hide unknown outcomes or collapse signing, broadcast and settlement into one success value. Application-specific builders can be separate modules or repositories and must produce a typed operation the Node can independently validate.

## Protocol objects

| Object | Meaning | Does not prove |
|---|---|---|
| Principal | Authenticated client, operator or Node identity | Permission by identity alone |
| Intent | Typed requested operation with scope and validity | Evaluation or execution |
| Policy | Versioned constraints and delegated capability | That Control can widen local limits |
| Decision | Result bound to exact Intent, Policy and evidence | Signing or execution |
| Evidence | Typed record with subject, issuer and state | More than its evidence type states |

Proposed logical operations are `Describe`, `Evaluate`, `Execute`, `GetReceipt`, and `Observe`. The envelope binds schema version, request ID, Principal, resource scope, target Node and epoch, Policy revision, validity window, exact typed action, and delivery mode. Duplicate JSON keys, unknown fields, unsupported versions, expired authority and inconsistent derived values are rejected.

Never expose a generic `sign(hash)` operation. A signing Node reconstructs the signable payload from a typed request and compares all derived values before accessing a Key Provider.

## Execution and evidence

The execution owner validates and durably reserves replay, budget and nonce state before signing. The Key Provider exposes a narrow local signing interface and never submits to a network. An optional delivery adapter may return stored signed bytes or submit exactly those bytes to an operator-configured RPC profile.

Evidence remains separate:

- signing: `pending`, `denied`, `signing_unknown`, `signed`;
- delivery: `not_requested`, `pending`, `submission_unknown`, `rpc_accepted`, `rpc_rejected`;
- observation: `unobserved`, `included`, `reorged`, `finalized`.

A timeout is an unknown outcome. Retry with the same request identity or retrieve its receipt. Never construct a different transaction or reuse a nonce merely because a response was lost.

## Transport and latency

HTTP is the implemented local transport. Core NATS and JetStream are possible adapters. Transport acknowledgements do not replace application replay state, local journal commits or chain reconciliation. JetStream KV does not provide a transaction spanning nonce, budget and replay fields.

The target is one client-to-execution-owner request/result exchange after authorization. Broadcast adds an owner-to-RPC exchange without requiring a return through Control. Compare transports only with equivalent validation, persistence, key backend and failure guarantees. Record p50, p95, p99, throughput, queue time and unknown outcomes before making performance claims.

## Current implementation and next step

[Budget Policy Node v0](budget-node-v0.md) implements keyless reservation accounting behind Control forwarding. It is one configured local Node, not fleet routing. The public SDK, Node descriptors, capability discovery and general operation state model are not implemented yet.

The next implementation boundary is the versioned SDK contract for `Describe`, operation submission and `GetReceipt`, followed by fleet registration and routing. Signing remains disabled until a typed EVM action, nonce ownership and durable signing receipt are implemented and independently tested.
