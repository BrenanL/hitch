# internal/tokens

Canonical token estimation for Hitch. All components that need to estimate token counts from character counts should use this package rather than implementing the heuristic inline.

## The Heuristic

`Estimate(charCount int) int` applies the chars/4 rule:

- Base estimate: `charCount / 4`
- Rounding: if `charCount % 4 >= 2`, round up by 1; otherwise truncate

This matches the widely-used rule of thumb that one token is approximately four characters of English text.

## Accuracy

This is a rough heuristic, not a tokenizer-based count. Actual token counts vary by:

- Language (non-English text often tokenizes less efficiently)
- Content type (code, JSON, and symbols may tokenize differently from prose)
- Model tokenizer version

Expect accuracy within roughly ±20% for typical mixed content. Do not use this for billing calculations — use actual token counts from API responses where available.

## Usage

```go
import "github.com/BrenanL/hitch/internal/tokens"

estimated := tokens.Estimate(len(bodyText))
```
