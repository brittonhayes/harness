Work an alert through vala's governed incident-response loop.

Use this when you have an alert that may need real-world action (notifying
responders, containment). It drives the alert through phases —
plan → evidence → propose → approval → execute → report — where the agent gathers
evidence read-only, proposes explicit actions citing that evidence, and only
executes an action after it is approved. You cannot shortcut this: side-effecting
actions are unavailable until the execute phase and only run with an approval on
record. The result is an auditable case (Evidence, Actions, and a narrative page)
in the brain.

Provide the alert either as a `path` to an alert JSON file
(`{alert_id, source, severity, raw}`) or with the inline `source`/`severity`/`raw`
fields. The tool returns a summary of the case: the phase reached and the
evidence, actions, and executions recorded.
