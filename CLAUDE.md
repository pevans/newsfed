All specifications are located in the `specs/` directory.

Do not commit files directly into the repository unless given explicit
permission to do so by the user's own prompt to you.

# Go programming

Unit tests should always focus on the behavior of the function or method being
tested. Use the `testify` assertion library, and focus on property tests
rather than exhaustive example-based tests.

Anytime you want to use `interface{}` as a Go type, you should use `any`
instead.

Whenever you change a test, you must run the test before you can say you're
done.

# Black box tests

Black box tests are located in `tests/` and are implemented with a tool called
`bats`.

Anytime you change a bats file in `tests/`, you must re-run the file with bats
to ensure your changes work.

Example: `bats tests/testfile.bats`
