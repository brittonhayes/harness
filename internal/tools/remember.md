Save a durable fact about this environment to VALA.md, vala's operator-memory
file, so it primes every future session instead of being re-learned each time.

Use it when a hunt teaches you standing context worth keeping: where a log
source lives, a known-good baseline ("svc-deploy rotates keys nightly ~02:00
UTC"), a detection or asset naming convention, a crown-jewel system, or a
confirmed environment quirk that explains what "normal" looks like.

Do NOT use it for hunt evidence (use record_finding) or threat intelligence —
indicators, TTPs, actors — (use record_intel); those are the hunt's output.
remember is only for durable background about the environment itself.

Keep each fact a single, specific sentence. Never store secrets or credentials.
