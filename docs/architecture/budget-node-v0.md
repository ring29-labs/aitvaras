# Budget Policy Node v0

- Accepted boundary: application clients are outside the Aitvaras Node fleet.
- Local implementation scope: Control forwarding plus a keyless `budget_policy_v0` Node.
- Proposed next steps: typed transaction reconstruction, signing, nonce ownership and settlement reconciliation.
- Deferred: mainnet, MPC, shared-Vault multi-owner budgets, production identity and JetStream integration.

## Outcome and boundary

A client can reserve separate spend and gas allowances and retry after a lost response without reserving twice. Control routes a request; the Node alone owns its configured limits and reservation journal. Neither a reservation nor its receipt is permission to sign or evidence of execution. Hosted and self-hosted use the same code; independent administration, not separate processes alone, establishes operator independence.

## First local profile

One Node serves one operator-provisioned policy, client, wallet, EVM network and spend asset. Policy configuration is a local JSON file; there is no remote create/update/reset endpoint. The application receives only the Control client token. Control holds a different Node token. The legacy development/admin token cannot access the budget API. These are local static credentials, not production enrollment or multi-tenant authentication.

Control exposes `POST /v1/budget/reservations`, `GET /v1/budget/reservations/{id}` and `GET /v1/budget`. It forwards to one fixed loopback Node URL, never a client URL or self-registered node. Node exposes the same endpoints using its separate credential. One HTTP exchange per hop, no polling. Control keeps no competing budget counter. Timeout means unknown outcome: query or retry the same request ID and fields.

Amounts are canonical unsigned base-unit integer strings, never floating point. The spend allowance is cumulative gross spend of the configured asset, not a balance or reusable capital. The current schema retains the field name `trade_limit`. Gas reservation is `gas_limit * max_fee_per_gas` in native wei; it does not include L1 data/blob fees or other network-specific fees. Real signing must add those fee models before using this counter as a full cost bound. Native-asset spend and gas are separate allocations; their combined balance impact is not checked yet.

Both reservations commit together. The complete normalized request binds request ID, policy ID, wallet, network, asset, amount and gas fields. Identical retries return the same receipt; ID reuse with changed fields conflicts. No automatic expiry, refund, release, budget increase or window reset is implemented. Denials do not reserve funds. No arbitrary calldata, transaction bytes, nonce, signature or broadcast is accepted.

## Persistence and failure semantics

An explicitly initialized local append-only journal stores a fixed policy header and accepted requests. An OS advisory exclusive file lock prevents two cooperating processes using the same file. Appends and file sync complete before a reservation is returned; replay validates every request and rebuilds both counters. Changed policy, malformed/truncated journal, unavailable storage or exhausted journal capacity fails closed. An uncertain write poisons the running ledger until operator recovery; it must never continue with an uncertain counter.

Normal startup requires an existing nonempty journal. Initialization is a separate `-init` action that refuses to overwrite any existing file. A missing journal must not silently reset authority. Backups, malicious rollback detection, remote filesystems, fencing against copied journals, automated recovery and multi-host failover are not implemented. Keep the journal on a local filesystem; a copied journal is not a new independent budget. A trusted operator can change code or files: this is not protection from that operator.

## Verification and handoff

Run the repository tests for boundary authentication, strict JSON, scope denial, separate gas/spend exhaustion, atomic concurrent accounting, idempotency/conflict, restart replay, changed-policy refusal and corrupt/missing journal refusal. Passing local tests is not signing, staging or production evidence.

Next: bind an allowlisted typed operation to independently reconstructed spend and total fees, then co-locate this budget engine with the signer execution owner. Do not trust a client-declared amount when signing actual calldata. Add nonce reservation and durable signing/reconciliation before enabling either signed-byte return or Node broadcast. JetStream can replace the forwarding transport without owning cross-field accounting.
