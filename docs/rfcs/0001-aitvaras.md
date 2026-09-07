# RIP-0001: Control plane, SDK, and node fleet

- Date: 2026-09-07
- Status: core scope accepted; protocol details proposed

## Accepted

1. Aitvaras is an open-source control plane and SDK for a fleet of independently operated execution Nodes.
2. Application clients are outside the Node fleet and outside the core repository. They integrate through SDK contracts.
3. Control manages discovery, routing, operation state and evidence. It cannot widen a Node's local policy.
4. Node roles are declared, versioned capabilities. Policy and signing are the first capability families.
5. Hosted and self-hosted deployments use the same protocol and validation semantics.
6. Registration, policy evaluation, signing, broadcast, inclusion, finality and settlement remain distinct states.
7. Avoidable round trips may be removed only while preserving the same enforcement and durability guarantees.

## Trust boundary

Control is not sufficient by itself to exercise Node-held authority. A Node verifies exact typed inputs, scope, policy revision, validity, replay state and locally configured constraints. A Signer Node reconstructs the signable payload and never accepts blind hashes as the complete authorization input.

A fleet is not a pool of interchangeable signing workers. One execution owner controls nonce, replay, budget and result state for each chain/address lane. Automatic failover requires fencing the previous owner from key access.

## Implemented

[Budget Policy Node v0](../architecture/budget-node-v0.md) provides local Control forwarding to one configured keyless Node with separate authentication. The Node durably and idempotently reserves spend and gas allowances and fails closed on malformed, conflicting, exhausted or unavailable state.

The receipt status is `reserved_simulation`. No signing, broadcast, funds movement, fleet routing or production identity is implemented.

## Proposed

Versioned protocol schemas will drive a public client SDK and Node SDK. The protocol begins with capability description, typed operation submission and receipt retrieval. Fleet registration adds authenticated Node descriptors, enrollment epochs, scope, supported schema versions, readiness and quarantine state.

HTTP remains the initial transport. Other transports must preserve the same idempotency, error and evidence semantics. Transport delivery is not domain authorization.

See [the architecture](../architecture/aitvaras.md).

## Deferred

Production workload identity, signing keys, MPC, multi-host fencing, policy amendment, network submission and mainnet operation require separate specifications and verification.
