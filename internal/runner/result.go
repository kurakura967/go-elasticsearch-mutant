package runner

// Status represents the outcome of running a mutant against the test suite.
type Status int

const (
	Killed   Status = iota // test failed  — mutant was detected
	Survived               // test passed  — mutant was not detected
	Timeout                // execution exceeded the time limit
	Error                  // build failure or other execution error
	Skipped                // operator skipped this mutation (e.g. would produce invalid code)
)

func (s Status) String() string {
	switch s {
	case Killed:
		return "KILLED"
	case Survived:
		return "SURVIVED"
	case Timeout:
		return "TIMEOUT"
	case Error:
		return "ERROR"
	case Skipped:
		return "SKIPPED"
	default:
		return "UNKNOWN"
	}
}

// Result holds the outcome of running a single mutant.
type Result struct {
	MutantID    int
	Status      Status
	Output      string       // human-readable go test output
	TestResults []TestResult // per-test pass/fail outcomes parsed from go test -json
}
