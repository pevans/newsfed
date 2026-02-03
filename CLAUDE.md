All RFCs are located in the `rfcs/` directory.

Whenever you have a task you must perform, you must track it with `bd`.

Do not commit files directly into the repository unless given explicit
permission to do so by the user's own prompt to you.

# Go programming

Unit tests should always focus on the behavior of the function or method being
tested. Use the `testify` assertion library, and focus on property tests
rather than exhaustive example-based tests.

Anytime you want to use `interface{}` as a Go type, you should use `any`
instead.
