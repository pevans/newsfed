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

# Black Box Tests

All black box tests are written with bash scripts. In the testing directory
there is a script named `run-tests.sh` that recursively runs the bash scripts
in each subdirectory, and prints the subdirectory that it tests as it goes
with either "OK" (if everything worked) or "FAIL" (if something went wrong).

- For tests of a CLI app, you must run the CLI app with appropriate input and
  test that it provides appropriate output or side effects. You must also test
  that its exit code is what you expect to see.
- For tests of an HTTP service, you must run `curl` with appropriate input and
  test for appropriate output and status codes.

Each test you write should have its own bash script and should be organized
under an appropriate subdirectory that groups the tests by the software being
tested.

Each test must not print anything out unless it fails. If it fails, it should
print the filename and some descriptive message of what went wrong.

Every test you add should be logged in @tests.v2/coverage.yaml, which tags
tests for the RFC and section that the test covers.

Example:

- `foo/bar.sh` tests that `bar` behavior of `foo` is correct. `foo` could be
  an HTTP service or it could be a CLI app.
