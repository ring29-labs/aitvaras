# Governance

Aitvaras uses lightweight, evidence-oriented open-source governance.

## Decision process

Material protocol, security, scope and governance changes start as a Ring29 Improvement Proposal under `docs/rfcs/`. A proposal records its status, trust assumptions, compatibility impact, objections and verification requirements.

Security-critical changes require:

- a written threat-model delta;
- denial and failure-path tests;
- review by someone other than the author;
- an explicit statement of what was and was not verified.

## Contributions

Contributions must keep implementation, locally tested behavior, staged behavior and production evidence distinct. Node registration, policy evaluation, approval, signing, broadcast and settlement are not interchangeable states.

Agents may research, propose, test and implement. They do not gain merge authority, signing authority or access to secrets by participating. Maintainers approve releases and changes to trust assumptions.
