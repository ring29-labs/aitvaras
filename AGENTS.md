# Agent working agreement

Before making changes, read `INTENT.md`, `ORIGIN.md`, and relevant RFCs.

Keep material architecture decisions reviewable under `docs/architecture/` and `docs/rfcs/`. Label accepted, implemented, proposed and deferred behavior explicitly.

For every contribution:

1. state the outcome and trust boundary;
2. keep hosted and self-hosted protocol behavior aligned;
3. never add credentials, keys, key shares, recovery material or customer data;
4. never copy private or incompatibly licensed source material;
5. distinguish mocked, locally tested, staged and production-proven behavior;
6. test denial and malformed-input paths, not only success paths;
7. leave a concise handoff in the issue or pull request.

Do not describe registration, policy evaluation, approval, signing, broadcast and settlement as interchangeable. Evidence for one is not evidence for the others.
