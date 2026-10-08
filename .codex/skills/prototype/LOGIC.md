# Interactive logic prototype

Use a self-contained HTML file only when pressing buttons and seeing transitions
will answer the question better than a smaller table, diagram, or script. Explain
the question visibly, use domain language, and display relevant state after every
action. Include the awkward transition or counterexample the decision depends on.
Keep logic separate from rendering where that improves clarity; no framework or
backend is needed for an in-memory experiment. A guided sequence is useful when
the user needs help reaching the interesting state, not a required UI feature.

Check the cases needed to trust the observation and report limits. Capture the
question, artifact and verdict following [the prototype skill](SKILL.md). Even
reusable-looking logic remains experimental; do not lift it into production unless
implementation is authorized and its normal checks are satisfied.
