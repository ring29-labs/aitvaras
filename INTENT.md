# Project scope

Aitvaras is an open-source control plane and SDK for coordinating independently operated execution nodes.

The repository focuses on three boundaries:

- Control manages node discovery, capability routing, operation state and evidence.
- The SDK defines stable contracts for clients and node implementations.
- Nodes independently enforce their configured policy and perform only declared capabilities.

Clients are external to the node fleet. Application-specific clients, including trading bots, own their domain logic and integrate through the SDK. They are not part of the Aitvaras core.

## Core vocabulary

| Term | Meaning |
|---|---|
| Control | Coordinates clients, operations and the node fleet |
| Node | Independently operated runtime exposing declared capabilities |
| Policy Node | Evaluates constraints without necessarily holding a key |
| Signer Node | Applies local policy and exposes a narrow signing capability |
| Principal | Authenticated identity scoped to a client, operator or node |
| Intent | Typed request for an operation |
| Policy | Versioned constraints governing an operation |
| Decision | Policy result bound to one exact request and policy revision |
| Evidence | Typed record of a specific processing or execution state |

## Current implementation

The current local slice provides:

- an HTTP Control service;
- node registration and typed intent compatibility handlers;
- Control forwarding to one configured Budget Policy Node;
- durable, idempotent reservation of separate spend and gas allowances;
- strict input, denial, concurrency and restart tests.

This slice does not sign, broadcast or move funds. Registration is not attestation, a policy reservation is not signing authorization, and RPC acceptance would not prove settlement.

## Engineering principles

- Deny malformed, ambiguous and out-of-scope requests.
- Keep node-local policy authoritative over Control requests.
- Bind decisions and evidence to exact versioned inputs.
- Make retries idempotent and unknown outcomes recoverable.
- Keep signing, broadcast, inclusion, finality and settlement distinct.
- Keep hosted and self-hosted behavior protocol-compatible.
- Remove unnecessary network hops without removing independent checks.
- Make performance and security claims only from corresponding evidence.
