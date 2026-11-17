## Phase 1, Step 2: fix the sentinel in `baskets`

The root cause is two declarations of the same error. The repository's package
declares its own copies, while the projector and the repository interface use
the ones in `catalog`. The fix is to delete the duplicates and return the
`catalog` ones. Two edits in the same file:

```go
// depot-baskets/baskets/internal/postgres/catalog_repository.go (delete this block)
var (
	ErrProductNotFound        = errors.New("product not found in catalog")
	ErrPriceAdjustmentSkipped = errors.New("price adjustment skipped: already applied or product not found")
)
```

```go
// depot-baskets/baskets/internal/postgres/catalog_repository.go (change in AdjustProductPrice)
	if tag.RowsAffected() == 0 {
		return catalog.ErrPriceAdjustmentSkipped // changed: was the package-local ErrPriceAdjustmentSkipped
	}
```

### What's happening here

The `catalog` package defines `Repository`, the interface the projector depends
on, along with the errors that interface can return. The `postgres` package is
an adapter that implements it, and it already imports `catalog` for
`catalog.Product`. The rule that makes this safe is that the owner of the
interface owns its error values, and adapters return them as-is. You already
follow that in `stores`: the Postgres repositories return
`domain.ErrStoreNotFound`, and `mapError` matches on it. The `baskets` catalog
repository was half-converted: `FindProductByID` already returns
`catalog.ErrProductNotFound`, but `AdjustProductPrice` returned a local copy.
That also means `postgres.ErrProductNotFound` is a second dead duplicate, which
we remove in the same edit.

Go doesn't catch this, because declaring two package-level variables with equal
`errors.New` strings is perfectly valid. Each `errors.New` call creates a
distinct value, and `errors.Is` and `==` compare identity, not text. That's the
reason the log lines would have looked fine while the control flow was wrong.
Because the failing branch only runs on a redelivery or an out-of-order
delivery, which is rare, the bug could have lived for months.

Note that `Project` in `catalog/project.go` still compares with
`err != ErrPriceAdjustmentSkipped`. That works now (the repository returns the
value unwrapped), but it would break the moment someone wraps the error with
`%w`. We'll rewrite that block in Step 5, so I'll switch it to `errors.Is` then
and not touch it twice.

### Run it

Before the test, check that nothing else referenced the deleted names:

```bash
grep -rn "ErrProductNotFound\|ErrPriceAdjustmentSkipped" depot-baskets/baskets --include='*.go'
```

I expect hits only in `catalog/repository.go`, `catalog/project.go`,
`internal/postgres/catalog_repository.go` and the new test. If another file
refers to `postgres.ErrProductNotFound`, the build will tell us, and the
compiler error will name it.

Then, from `depot-baskets/baskets`:

```bash
go build ./... && go vet ./...
go test ./internal/postgres/... -run AdjustProductPrice -v -timeout 30s
```

Expect a clean build and both subtests passing: `--- PASS` for the parent and
for `redelivery_of_the_same_sequence` and `older_sequence_arriving_late`. The
running `baskets` container still has the old code, but behavior only differs on
redelivery, so we'll rebuild once at the end of the phase and not after every
step.

Paste the grep output and the test result. Any questions about the "interface
owner owns the errors" rule, or about why the test sits at the repository level
and not the projector level? Once it's green, Step 3 applies the same test and
fix to `depot`.
