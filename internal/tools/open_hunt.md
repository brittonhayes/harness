Open a hypothesis-driven threat hunt and make it the active hunt for this session.

Call this first when you start hunting a threat question. It creates the hunt in
the brain and lets `record_finding` and `store_hunt` write to it. Pass the
`question` (required) and, if you have one, a `hypothesis` and related `mitre`
technique.

After opening: investigate with read-only tools (`log_search`, `read`, `grep`,
`glob`), record each fact you rely on with `record_finding` (cite the returned
ID), surface reusable intel with `record_intel`, then call `store_hunt` once with
a Confirmed / Refuted / Inconclusive verdict.
