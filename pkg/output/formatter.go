package output

import "github.com/sw33tLie/uff/v2/pkg/ffuf"

// OutputFormatter defines the strategy for formatting output
type OutputFormatter interface {
	Printf(format string, a ...interface{})
	Println(a ...interface{})
	Banner(options map[string]string)
	Result(res ffuf.Result)
	Warning(msg string)
	Error(msg string)
	Info(msg string)
	Finalize() error
}
