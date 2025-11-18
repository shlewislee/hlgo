package hlgo

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

type Command struct {
	client      *Hledger
	baseCommand string
	args        []string

	final []string
	once  sync.Once
}

type Option func(*Command)

// NewCommand returns a [*Command] instance with the given options applied.
//
// command can be an empty string(e.g. to use --version flag)
func (client *Hledger) NewCommand(command string, options ...Option) *Command {
	cmd := &Command{
		baseCommand: command,
		client:      client,
	}

	for _, opt := range options {
		opt(cmd)
	}
	return cmd
}

type CommandError struct {
	Command string
	Stderr  string
	Err     error
}

func (e *CommandError) Error() string {
	return fmt.Sprintf("hledger command %q failed: %v", e.Command, e.Err)
}

func (e *CommandError) Unwrap() error {
	return e.Err
}

// Run executes the hledger command and captures its stdout and stderr. It returns the stdout output as a byte slice.
//
// If the command fails, it returns a [*CommandError] containing the command's stderr.
func (c *Command) Run(ctx context.Context) ([]byte, error) {
	cmd := c.build(ctx)

	var stdOutBuf, stdErrBuf bytes.Buffer

	cmd.Stdout = &stdOutBuf
	cmd.Stderr = &stdErrBuf

	if err := cmd.Run(); err != nil {
		return nil, &CommandError{
			Command: cmd.String(),
			Stderr:  stdErrBuf.String(),
			Err:     err,
		}
	}

	return stdOutBuf.Bytes(), nil
}

// Stream executes the hledger command and writes its stdout directly to the provided io.Writer.
// This is typically used for generating large reports (like CSV or JSON) to avoid buffering the entire output into memory.
//
// If the command fails, it returns a [*CommandError] containing the command's stderr.
func (c *Command) Stream(ctx context.Context, w io.Writer) error {
	var stdErrBuf bytes.Buffer

	cmd := c.build(ctx)

	cmd.Stdout = w
	cmd.Stderr = &stdErrBuf

	if err := cmd.Run(); err != nil {
		return &CommandError{
			Command: cmd.String(),
			Stderr:  stdErrBuf.String(),
			Err:     err,
		}
	}
	return nil
}

// String returns the full shell command string.
// It includes the binary path, default arguments, and command options.
// Uses context.TODO() during build. Intended for **debugging, logging, and fmt.Stringer**.
func (c *Command) String() string {
	return c.build(context.TODO()).String()
}

func (c *Command) build(ctx context.Context) *exec.Cmd {
	c.once.Do(func() {
		for _, opt := range c.client.DefaultArgs {
			opt(c)
		}
		if len(c.client.JournalPaths) != 0 {
			for _, p := range c.client.JournalPaths {
				c.final = append(c.final, "-f", p)
			}
		}
		if c.baseCommand != "" {
			c.final = append(c.final, c.baseCommand)
		}
		c.final = append(c.final, c.args...)
	})
	cmd := exec.CommandContext(ctx, c.client.Binary, c.final...)
	return cmd
}

// --- Command Options ---

// WithArg will simply append given string arguments.
// Use [WithArg] if a specific argument option is not implemented by the library.
func WithArg(arg ...string) Option {
	return func(cmd *Command) {
		cmd.args = append(cmd.args, arg...)
	}
}

func WithAccount(account ...string) Option {
	accounts := make([]string, 0, len(account))
	for _, acct := range account {
		var sb strings.Builder
		sb.WriteString(`acct:`)
		sb.WriteString(acct)

		accounts = append(accounts, sb.String())
	}
	return func(cmd *Command) {
		cmd.args = append(cmd.args, accounts...)
	}
}

type AccountType string

const (
	AccountTypeAsset      AccountType = "A"
	AccountTypeLiability  AccountType = "L"
	AccountTypeEquity     AccountType = "E"
	AccountTypeRevenue    AccountType = "R"
	AccountTypeExpense    AccountType = "X"
	AccountTypeCash       AccountType = "C"
	AccountTypeConversion AccountType = "V"
)

func WithAccountTypes(acctTypes ...AccountType) Option {
	var sb strings.Builder
	sb.WriteString("type:")
	for _, at := range acctTypes {
		sb.WriteString(string(at))
	}
	return func(cmd *Command) {
		cmd.args = append(cmd.args, sb.String())
	}
}

func WithDate(dateStr string) Option {
	var sb strings.Builder
	sb.WriteString("date:")
	sb.WriteString(dateStr)
	return func(cmd *Command) {
		cmd.args = append(cmd.args, sb.String())
	}
}

// show only top `level` levels of accounts. If `filter` is set, only apply limiting to accounts matching the regular expression.
func WithDepth(level int, filter string) Option {
	var sb strings.Builder

	sb.WriteString("depth:")
	if filter != "" {
		sb.WriteString(filter)
		sb.WriteString("=")
	}
	sb.WriteString(strconv.Itoa(level))

	return func(cmd *Command) {
		cmd.args = append(cmd.args, sb.String())
	}
}

// Calculate with postings from journal start to column end, ie "all postings from before report start date until this column's end"
func WithHistorical() Option {
	return func(cmd *Command) {
		cmd.args = append(cmd.args, "--historical")
	}
}

type OutputType string

const (
	OutputTXT  OutputType = "txt"
	OutputCSV  OutputType = "csv"
	OutputHTML OutputType = "html"
	OutputTSV  OutputType = "tsv"
	OutputJSON OutputType = "json"
	OutputFODS OutputType = "fods"
)

// See https://hledger.org/1.50/hledger.html#output-format for supported formats. Note that this library will not check the availability.
func WithOutputType(outputType OutputType) Option {
	return func(cmd *Command) {
		cmd.args = append(cmd.args, fmt.Sprintf("-O=%s", outputType))
	}
}

type PeriodType string

const (
	PeriodDaily     PeriodType = "--daily"
	PeriodWeekly    PeriodType = "--weekly"
	PeriodMonthly   PeriodType = "--monthly"
	PeriodQuarterly PeriodType = "--quarterly"
	PeriodYearly    PeriodType = "--yearly"
)

// See https://hledger.org/1.50/hledger.html#period-expressions
func NewCmdPeriod(periodStr string) PeriodType {
	return PeriodType(fmt.Sprintf("--period=%s", periodStr))
}

// See https://hledger.org/1.50/hledger.html#report-start--end-date
func WithPeriod(p PeriodType) Option {
	return func(cmd *Command) {
		cmd.args = append(cmd.args, string(p))
	}
}

// With --pivot PIVOTEXPR, some other field's (or multiple fields') value is used as a synthetic account name, causing different grouping and display.
//
// See https://hledger.org/1.50/hledger.html#pivoting
func WithPivot(tagName string) Option {
	var sb strings.Builder
	sb.WriteString(`--pivot=`)
	sb.WriteString(tagName)

	return func(cmd *Command) {
		cmd.args = append(cmd.args, sb.String())
	}
}

type QueryType string

const (
	QueryTypeAcct  QueryType = "acct"  // acct:REGEX
	QueryTypeAmt   QueryType = "amt"   // amt:N, amt:'<N', amt:'<=N', amt:'>N', amt:'>=N'
	QueryTypeCode  QueryType = "code"  // code:REGEX
	QueryTypeCur   QueryType = "cur"   // cur:REGEX
	QueryTypeDesc  QueryType = "desc"  // desc:REGEX
	QueryTypeDate2 QueryType = "date2" // date2:PERIODEXPR
	QueryTypeNote  QueryType = "note"  // note:REGEX
	QueryTypePayee QueryType = "payee" // payee:REGEX
	QueryTypeReal  QueryType = "real"  // real:, real:0
	QueryTypeTag   QueryType = "tag"   // tag:NAMEREGEX[=VALREGEX]
)

// Check https://hledger.org/1.50/hledger.html#queries for more information.
func WithQuery(queryType QueryType, value string) Option {
	var sb strings.Builder
	sb.WriteString(string(queryType))
	sb.WriteString(":")
	sb.WriteString(value)
	return func(cmd *Command) {
		cmd.args = append(cmd.args, sb.String())
	}
}

// WithNotQuery prepends `not:` to a query to negate the match.
func WithNotQuery(queryType QueryType, value string) Option {
	var sb strings.Builder
	sb.WriteString("not:")
	sb.WriteString(string(queryType))
	sb.WriteString(":")
	sb.WriteString(value)
	return func(cmd *Command) {
		cmd.args = append(cmd.args, sb.String())
	}
}

type TrStatusType string

const (
	TrStatusUnmarked TrStatusType = ""
	TrStatusPending  TrStatusType = "!"
	TrStatusCleared  TrStatusType = "*"
)

// Match unmarked, pending, or cleared transactions respectively.
func WithStatus(trStatus TrStatusType) Option {
	var sb strings.Builder
	sb.WriteString("status:")
	sb.WriteString(string(trStatus))
	return func(cmd *Command) {
		cmd.args = append(cmd.args, sb.String())
	}
}

// You can also do
//
//	WithValuation(ValuationType("YYYY-mm-dd"))
//
// to use custom date.
type ValuationType string

const (
	// Convert amounts to their value in the default valuation commodity using current market prices (as of when report is generated).
	ValuationNow ValuationType = "now"
	// Convert amounts to their value in the default valuation commodity, using market prices on the last day of the report period (or if unspecified, the journal's end date); or in multiperiod reports, market prices on the last day of each subperiod.
	ValuationEnd ValuationType = "end"
	// Convert amounts to their value in the default valuation commodity, using market prices on each posting's date.
	ValuationThen ValuationType = "then"
)

// show amounts converted to their value on the specified date(s) in their default valuation
func WithValuation(commodity string, valType ValuationType) Option {
	return func(cmd *Command) {
		argument := fmt.Sprintf("--value=%s,%s", valType, commodity)
		cmd.args = append(cmd.args, argument)
	}
}

type TxnBalancingType string

const (
	TxnBalancingOld   TxnBalancingType = "old"
	TxnBalancingExact TxnBalancingType = "exact"
)

func WithTxnBalancing(balancingType TxnBalancingType) Option {
	return func(cmd *Command) {
		cmd.args = append(cmd.args, fmt.Sprintf("--txn-balancing=%s", balancingType))
	}
}

// descPattern is optional. It will only parse the very first argument
func WithBudget(descPattern ...string) Option {
	var sb strings.Builder
	sb.WriteString("--budget")
	if len(descPattern) != 0 {
		sb.WriteString("=")
		sb.WriteString(descPattern[0])
	}
	return func(cmd *Command) {
		cmd.args = append(cmd.args, sb.String())
	}
}

// Period is optional. It will only parse the very first argument.
func WithForecast(period ...string) Option {
	var sb strings.Builder
	sb.WriteString("--forecast")
	if len(period) != 0 {
		sb.WriteString("=")
		sb.WriteString(period[0])
	}
	return func(cmd *Command) {
		cmd.args = append(cmd.args, sb.String())
	}
}

func WithAlias(a, b string) Option {
	var sb strings.Builder
	sb.WriteString("--alias=")
	sb.WriteString(a)
	sb.WriteString("=")
	sb.WriteString(b)

	return func(cmd *Command) {
		cmd.args = append(cmd.args, sb.String())
	}
}

func WithToday(date string) Option {
	var sb strings.Builder
	sb.WriteString("--today=")
	sb.WriteString(date)
	return func(cmd *Command) {
		cmd.args = append(cmd.args, sb.String())
	}
}

func WithCommodityStyle(style string) Option {
	var sb strings.Builder
	sb.WriteString("--commodity-style=")
	sb.WriteString(style)
	return func(cmd *Command) {
		cmd.args = append(cmd.args, sb.String())
	}
}

func WithDrop(n int) Option {
	return func(cmd *Command) {
		dropN := fmt.Sprintf("--drop=%v", n)
		cmd.args = append(cmd.args, dropN)
	}
}

func WithFormat(format string) Option {
	return func(cmd *Command) {
		cmd.args = append(cmd.args, "--format", format)
	}
}

func WithPretty() Option {
	return func(cmd *Command) {
		cmd.args = append(cmd.args, "--pretty")
	}
}

func WithTree() Option {
	return func(cmd *Command) {
		cmd.args = append(cmd.args, "--tree")
	}
}

func WithInvert() Option {
	return func(cmd *Command) {
		cmd.args = append(cmd.args, "--invert")
	}
}

func WithPercent() Option {
	return func(cmd *Command) {
		cmd.args = append(cmd.args, "--percent")
	}
}

// show a row total column
func WithRowTotal() Option {
	return func(cmd *Command) {
		cmd.args = append(cmd.args, "--row-total")
	}
}

func WithAverage() Option {
	return func(cmd *Command) {
		cmd.args = append(cmd.args, "--average")
	}
}

func WithNoTotal() Option {
	return func(cmd *Command) {
		cmd.args = append(cmd.args, "--no-total")
	}
}

func WithSortAmount() Option {
	return func(cmd *Command) {
		cmd.args = append(cmd.args, "--sort-amount")
	}
}

func WithInferMarketPrice() Option {
	return func(cmd *Command) {
		cmd.args = append(cmd.args, "--infer-market-price")
	}
}

func WithCumulative() Option {
	return func(cmd *Command) {
		cmd.args = append(cmd.args, "--cumulative")
	}
}

func WithCount() Option {
	return func(cmd *Command) {
		cmd.args = append(cmd.args, "--count")
	}
}

func WithValueChange() Option {
	return func(cmd *Command) {
		cmd.args = append(cmd.args, "--valuechange")
	}
}

func WithDeclared() Option {
	return func(cmd *Command) {
		cmd.args = append(cmd.args, "--declared")
	}
}

func WithEmpty() Option {
	return func(cmd *Command) {
		cmd.args = append(cmd.args, "--empty")
	}
}

func WithCost() Option {
	return func(cmd *Command) {
		cmd.args = append(cmd.args, "--cost")
	}
}

func WithIgnoreAssertions() Option {
	return func(cmd *Command) {
		cmd.args = append(cmd.args, "--ignore-assertions")
	}
}

func WithAuto() Option {
	return func(cmd *Command) {
		cmd.args = append(cmd.args, "--auto")
	}
}

func WithStrict() Option {
	return func(cmd *Command) {
		cmd.args = append(cmd.args, "--strict")
	}
}
