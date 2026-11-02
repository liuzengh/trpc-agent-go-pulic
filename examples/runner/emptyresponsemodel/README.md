# Empty model response retries with Runner + LLMAgent

This example rejects a completed response whose first choice has no `Content`,
`ContentParts`, or `ToolCalls`. A reasoning-only response is rejected. It retries
at most `max-retries` additional times and keeps rejected attempts out of session
history. The first choice matches the choice consumed by LLMAgent. Whitespace is
preserved as content; this example does not implement response quality scoring.

This variant wraps `model.Model` in application code. The implementation in
`retry.go` validates an attempt before returning any response to LLMAgent.
`main.go` shows the same Runner and Session wiring used by the callback variant.

## Run

Use Go 1.24.4 or newer, as required by the examples module. Configure an
OpenAI-compatible provider through environment variables:

```bash
export OPENAI_API_KEY="your-key"
export OPENAI_BASE_URL="https://your-provider.example/v1"
export MODEL_NAME="your-model"
cd examples/runner/emptyresponsemodel
go run . -prompt 'Reply with OK.' -streaming=true -max-retries=2
```

If a local `glm.sh` exports these variables, source it in the same shell before
running the example or smoke test. The environment file and credentials are not
part of this example.

The command prints the answer and stored conversation. Use `-streaming=false`
for non-streaming provider requests. `-model` overrides `MODEL_NAME`.
`-retry-delay` defaults to 250 ms; `-timeout` defaults to two minutes for the
entire run. `-max-retries=0` disables application retries. Negative retries/delay
and non-positive timeouts are rejected. SDK HTTP retries are explicitly disabled
so the retry count is easy to observe. Transport/API errors pass through without
application retries.

## History and failure handling

The wrapper takes a JSON snapshot of the request and restores a fresh copy for
each attempt. This follows the existing model failover approach while preserving
JSON numbers with `UseNumber`. It requires JSON-serializable request fields and
shares tool implementations, which model adapters must not mutate. Provider
request tailoring cannot change subsequent attempts or the caller's request.
Rejected responses stay inside the wrapper; no cleanup of session messages is
needed. All retry state is local to GenerateContent, so concurrent calls are
isolated when the underlying model is safe to share.

After exhaustion, the caller receives a terminal response with error type
`example_empty_model_response`. The Session `AppendEvent` hook filters only that
error type, so even the final rejection stays out of history. Other errors retain
the Runner's normal behavior. The user's message remains in the conversation.
The CLI exits unsuccessfully after displaying the terminal result and history.

Neither approach retries the entire `Runner.Run`, so the user message is not
inserted again and completed tools are not rerun. An accepted tool-call response
continues through the normal tool execution flow. Retry budgets reset after an
accepted completion, including a tool-call response.

## Comparison

| Behavior | AfterModel + CustomResponse | Application model wrapper |
| --- | --- | --- |
| Retry boundary | New LLMAgent model-call cycle | Inside one GenerateContent call |
| BeforeModel / model selection | Runs again | Runs once around the wrapper |
| Request | Rebuilt from session and callbacks | Fresh snapshot copy for each attempt |
| MaxLLMCalls | Counts each agent cycle | Counts the wrapper call; wrapper bounds its attempts |
| Streaming to the client | Partials are forwarded before final validation | Only the validated complete response is emitted |
| Failed complete responses in history | Removed by replacement with a non-content control response | Never emitted by the wrapper |
| Provider closes without a full response | No complete response to validate in this hook | Retried as an incomplete attempt |
| Token usage | Rejected completion usage is retained on control responses | Rejected attempt usage is not included in the emitted result |

Both examples live entirely in application/example code and leave framework
APIs and default behavior unchanged. They are alternatives for comparison; the
model wrapper makes stronger isolation guarantees at the cost of streaming
latency and visibility into rejected-attempt usage.

`GenerateContent` deliberately waits until validation finishes, then returns a
buffered channel containing one complete result. The provider can still stream,
but the wrapper discards partials and retains only the latest full response; it
does not buffer the entire stream. This requires an aggregated non-partial
response, as provided by model/openai. A partial-only or empty closed stream is
retried. A nil channel is treated as a provider contract error. The wrapper does
not implement or promote IterModel, so the flow cannot bypass validation through
an iterator. Each attempt owns a child context that is canceled on completion or
error, and receiving/backoff honor caller cancellation. The underlying adapter
must honor context cancellation too. Rejected attempts still incur provider
charges; production usage accounting should record them separately.

## Tests

From `examples/`:

```bash
go test ./runner/emptyresponsemodel
go vet ./runner/emptyresponsemodel
go build -o /tmp/emptyresponsemodel ./runner/emptyresponsemodel
```

Deterministic tests cover empty content, reasoning-only and choice-less responses,
streaming partials, retry exhaustion, zero retries, provider errors, concurrent
runs, cancellation, request/history cleanliness, and retries before and after a
tool call without repeating that tool. `TestRetryBoundary` asserts that two
provider attempts trigger one BeforeModel execution(s).

The live smoke test is opt-in and makes four real provider calls: two in each
streaming mode. It replaces the first completed attempt in each mode with a
reasoning-only response, then verifies that retrying produces an answer and that
both the retry prompt and session exclude the rejected attempt. It validates the
retry path deterministically instead of assuming the provider will fail:

```bash
# First export MODEL_NAME, OPENAI_API_KEY and OPENAI_BASE_URL (for example via glm.sh).
cd examples
TRPC_EMPTY_RESPONSE_SMOKE=1 go test ./runner/emptyresponsemodel -run TestLiveSmoke -v -count=1
```

Ordinary tests never call a live provider, even when credentials are present.

### Race detector on the fork baseline

The full `go test -race` run exposes existing races in the fork baseline
(`4022eeb14`): `Session.Clone` reads `UpdatedAt` while `UpdateUserSession` writes
it during tool execution, and `runner.ensureErrorEventContent` mutates an error
response while the flow reads it. The same races reproduce with plain
Runner + LLMAgent, without either retry implementation. These example-only
changes do not modify those framework paths. Normal tests, focused race checks
of the retry logic, vet, lint, and live smoke checks are run separately.
