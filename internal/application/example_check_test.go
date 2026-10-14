package application_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/thapelomagqazana/forge/internal/application"
	"github.com/thapelomagqazana/forge/internal/forgeerr"
)

// fakeCheckPort is a hand-written fake. It records calls and returns
// scripted results.
//
// A hand-written fake is preferred to a mocking framework: it is ~15 lines,
// it reads as plain Go, it has no reflection, and it introduces no
// dependency into the test package. See docs/development.md.
type fakeCheckPort struct {
	// scripted
	findings []application.Finding
	err      error

	// recorded
	calls  int
	root   string
	strict bool
}

func (f *fakeCheckPort) Check(_ context.Context, root string, strict bool) ([]application.Finding, error) {
	f.calls++
	f.root = root
	f.strict = strict
	return f.findings, f.err
}

// --- Positive ---

func TestCheckService_Run_HappyPath(t *testing.T) {
	port := &fakeCheckPort{
		findings: []application.Finding{
			{Path: "a.yaml", Message: "missing name", Severity: "warn"},
		},
	}
	svc := application.NewCheckService(port)

	out, err := svc.Run(context.Background(), application.CheckInput{
		Root:   "/proj",
		Strict: true,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(out.Findings) != 1 {
		t.Fatalf("findings = %d, want 1", len(out.Findings))
	}
	if port.calls != 1 || port.root != "/proj" || !port.strict {
		t.Fatalf("port not invoked as expected: %+v", port)
	}
}

func TestCheckService_Run_ZeroFindingsIsSuccess(t *testing.T) {
	svc := application.NewCheckService(&fakeCheckPort{findings: nil})

	out, err := svc.Run(context.Background(), application.CheckInput{Root: "/proj"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(out.Findings) != 0 {
		t.Fatalf("findings = %v, want empty", out.Findings)
	}
}

// --- Negative: caller contract violation ---

func TestCheckService_Run_EmptyRootIsCategoryCLI(t *testing.T) {
	// An empty Root is a usage error, not a check failure. Assert the
	// *category*, not just "some error", so that a future reclassification
	// of the code is caught here rather than at the exit-code layer.
	//
	// Note on the API: *forgeerr.Error.Category() returns a single
	// Category value. The two-value form (Category, bool) lives on
	// forgeerr.Code.Category() in codes.go, and is used only when the
	// caller has a bare code string that may not be registered. Once an
	// error has been constructed, its category is valid by construction
	// (validateCategory runs in New and Wrap), so no boolean is needed.
	svc := application.NewCheckService(&fakeCheckPort{})

	_, err := svc.Run(context.Background(), application.CheckInput{Root: ""})
	if err == nil {
		t.Fatal("expected error for empty root")
	}

	var fe *forgeerr.Error
	if !errors.As(err, &fe) {
		t.Fatalf("error is not *forgeerr.Error: %T", err)
	}

	if cat := fe.Category(); cat != forgeerr.CategoryCLI {
		t.Fatalf("category = %v, want CategoryCLI", cat)
	}
	if !strings.Contains(err.Error(), "root must not be empty") {
		t.Fatalf("message missing: %v", err)
	}
}

// --- Negative: port failure propagation ---

func TestCheckService_Run_BarePortErrorIsClassifiedAsInternal(t *testing.T) {
	// A bare error from the port has no category. The application layer
	// must classify it as CategoryInternal / CodeInternalUnexpected,
	// because the registry has no "other" member (see forgeerr doc).
	sentinel := errors.New("boom")
	port := &fakeCheckPort{err: sentinel}
	svc := application.NewCheckService(port)

	_, err := svc.Run(context.Background(), application.CheckInput{Root: "/proj"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, sentinel) {
		t.Fatalf("error chain lost sentinel: %v", err)
	}

	var fe *forgeerr.Error
	if !errors.As(err, &fe) {
		t.Fatalf("error is not *forgeerr.Error: %T", err)
	}
	if fe.Code() != string(forgeerr.CodeInternalUnexpected) {
		t.Fatalf("code = %q, want %q", fe.Code(), forgeerr.CodeInternalUnexpected)
	}
}

func TestCheckService_Run_ClassifiedPortErrorIsPreserved(t *testing.T) {
	// If the port already produced a *forgeerr.Error, the application
	// layer must NOT reclassify it. This is the "wrap is idempotent on
	// classified errors" contract from constructors.go, and it is what
	// lets safefs choose a specific filesystem or security code and have
	// the CLI see that code unchanged.
	portErr := forgeerr.New(
		forgeerr.CategoryFilesystem,
		forgeerr.CodeFilesystemNotFound,
		"missing input",
	)
	port := &fakeCheckPort{err: portErr}
	svc := application.NewCheckService(port)

	_, err := svc.Run(context.Background(), application.CheckInput{Root: "/proj"})
	if err == nil {
		t.Fatal("expected error")
	}

	// The exact *Error value must come through unchanged.
	if err != error(portErr) {
		t.Fatalf("error was reclassified:\n got: %v\nwant: %v", err, portErr)
	}

	var fe *forgeerr.Error
	if !errors.As(err, &fe) {
		t.Fatalf("error is not *forgeerr.Error: %T", err)
	}
	if fe.Code() != string(forgeerr.CodeFilesystemNotFound) {
		t.Fatalf("code = %q, want %q", fe.Code(), forgeerr.CodeFilesystemNotFound)
	}
}

// --- Edge / boundary ---

func TestCheckService_Run_ContextCanceledBeforeCall(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	port := &fakeCheckPort{}
	svc := application.NewCheckService(port)

	_, err := svc.Run(ctx, application.CheckInput{Root: "/proj"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if port.calls != 0 {
		t.Fatalf("port called %d times after cancel; want 0", port.calls)
	}
}

func TestCheckService_Run_ContextDeadlinePropagates(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 0)
	defer cancel()

	svc := application.NewCheckService(&fakeCheckPort{})
	_, err := svc.Run(ctx, application.CheckInput{Root: "/proj"})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want context.DeadlineExceeded", err)
	}
}

// --- Corner: constructor ---

func TestNewCheckService_PanicsOnNilPort(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on nil port")
		}
	}()
	_ = application.NewCheckService(nil)
}
