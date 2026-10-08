# Vue behavior changes

Also load the TypeScript reference when applicable. Follow the repository's Vue
version, existing component test stack, and SFC typecheck command (for example,
the configured vue-tsc script). Do not replace tooling or mount conventions.

Test rendered behavior and user interactions through the existing component or
browser boundary. Cover relevant props, emitted events, state changes, and visible
error/loading states; avoid tests coupled to private component methods. Await
DOM/reactivity updates and relevant resolved promises using the repository's
helpers before asserting. Observe the intended failing behavior before the fix,
then rerun it and affected tests. SFC type checking complements runtime rendering
tests; neither substitutes for the other. Use snapshots only where reviewing their
meaningful visible output adds evidence.
