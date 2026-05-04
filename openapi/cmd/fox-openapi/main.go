package main

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/fox-gonic/fox-openapi/internal/cli"
)

var version = "dev"

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		usage()
		return cli.ExitUsage
	}
	switch args[0] {
	case "generate":
		return runGenerate(args[1:])
	case "check":
		return runCheck(args[1:])
	case "serve":
		return runServe(args[1:])
	case "version":
		fmt.Println(version)
		return 0
	case "-h", "--help", "help":
		usage()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", args[0])
		usage()
		return cli.ExitUsage
	}
}

func runGenerate(args []string) int {
	cfg, code := parseCommon("generate", args)
	if code != 0 {
		return code
	}
	data, warnings, err := cli.RunPipeline(cfg)
	for _, warning := range warnings {
		fmt.Fprintln(os.Stderr, warning)
	}
	if err != nil {
		return handleError(err)
	}
	out := cli.ResolveOutputPath(cfg)
	if err := cli.WriteAtomic(out, data); err != nil {
		fmt.Fprintf(os.Stderr, "write %s: %v\n", out, err)
		return cli.ExitWriteFailed
	}
	return 0
}

func runCheck(args []string) int {
	cfg, code := parseCommon("check", args)
	if code != 0 {
		return code
	}
	data, warnings, err := cli.RunPipeline(cfg)
	for _, warning := range warnings {
		fmt.Fprintln(os.Stderr, warning)
	}
	if err != nil {
		return handleError(err)
	}
	out := cli.ResolveOutputPath(cfg)
	if err := cli.CheckDrift(out, data); err != nil {
		if errors.Is(err, cli.ErrDrift) {
			fmt.Fprintf(os.Stderr, "%s is out of date. Run `fox-openapi generate` to refresh.\n", out)
			return cli.ExitDrift
		}
		fmt.Fprintf(os.Stderr, "check %s: %v\n", out, err)
		return cli.ExitWriteFailed
	}
	fmt.Printf("%s is up to date.\n", out)
	return 0
}

func runServe(args []string) int {
	cfg, code, serveCfg := parseServe(args)
	if code != 0 {
		return code
	}
	if _, _, err := net.SplitHostPort(serveCfg.Addr); err != nil && strings.HasPrefix(serveCfg.Addr, ":") {
		serveCfg.Addr = "127.0.0.1" + serveCfg.Addr
	}
	if err := cli.Serve(cfg, serveCfg); err != nil {
		fmt.Fprintf(os.Stderr, "serve: %v\n", err)
		return cli.ExitUsage
	}
	return 0
}

func parseCommon(name string, args []string) (cli.Config, int) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	overrides, sources := bindCommonFlags(fs)
	if err := fs.Parse(args); err != nil {
		return cli.Config{}, cli.ExitUsage
	}
	fs.Visit(func(f *flag.Flag) { markOverride(overrides, f.Name) })
	overrides.Sources = sources.values
	overrides.SourcesSet = sources.set
	cfg, err := cli.LoadConfig(*overrides)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return cli.Config{}, cli.ExitUsage
	}
	return cfg, 0
}

func parseServe(args []string) (cli.Config, int, cli.ServeConfig) {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	overrides, sources := bindCommonFlags(fs)
	serveCfg := cli.ServeConfig{Addr: "127.0.0.1:8765", UIs: []string{"swagger"}, Watch: true}
	var ui repeatedFlag
	fs.StringVar(&serveCfg.Addr, "addr", serveCfg.Addr, "HTTP listen address")
	fs.Var(&ui, "ui", "UI to serve")
	fs.BoolVar(&serveCfg.Watch, "watch", serveCfg.Watch, "watch .go files and regenerate")
	fs.BoolVar(&serveCfg.Open, "open", false, "open browser")
	if err := fs.Parse(args); err != nil {
		return cli.Config{}, cli.ExitUsage, serveCfg
	}
	fs.Visit(func(f *flag.Flag) { markOverride(overrides, f.Name) })
	overrides.Sources = sources.values
	overrides.SourcesSet = sources.set
	if ui.set {
		serveCfg.UIs = ui.values
	}
	cfg, err := cli.LoadConfig(*overrides)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return cli.Config{}, cli.ExitUsage, serveCfg
	}
	return cfg, 0, serveCfg
}

func bindCommonFlags(fs *flag.FlagSet) (*cli.Overrides, *repeatedFlag) {
	o := &cli.Overrides{}
	var sources repeatedFlag
	fs.StringVar(&o.ConfigPath, "config", "fox-openapi.yaml", "config file path")
	fs.StringVar(&o.Entry, "entry", "", "entry function")
	fs.StringVar(&o.Out, "out", "api/openapi.yaml", "output path")
	fs.StringVar(&o.Format, "format", "", "yaml or json")
	fs.Var(&sources, "source", "source path")
	fs.BoolVar(&o.IncludeTestFiles, "include-test-files", false, "include *_test.go")
	fs.StringVar(&o.MetadataHook, "metadata-hook", "", "metadata hook")
	fs.BoolVar(&o.AutoAdd, "auto-add", false, "auto-add module requirement")
	fs.StringVar(&o.Workdir, "workdir", ".", "user project root")
	fs.BoolVar(&o.KeepDriver, "keep-driver", false, "keep generated driver")
	fs.BoolVar(&o.Verbose, "verbose", false, "verbose output")
	return o, &sources
}

func markOverride(o *cli.Overrides, name string) {
	switch name {
	case "config":
		o.ConfigExplicit = true
	case "entry":
		o.EntrySet = true
	case "out":
		o.OutSet = true
	case "format":
		o.FormatSet = true
	case "include-test-files":
		o.IncludeTestFilesSet = true
	case "metadata-hook":
		o.MetadataHookSet = true
	case "auto-add":
		o.AutoAddSet = true
	case "workdir":
		o.WorkdirSet = true
	case "keep-driver":
		o.KeepDriverSet = true
	case "verbose":
		o.VerboseSet = true
	}
}

func handleError(err error) int {
	var driverErr *cli.DriverError
	if errors.As(err, &driverErr) {
		fmt.Fprintln(os.Stderr, driverErr.Error())
		return driverErr.ExitCode
	}
	fmt.Fprintln(os.Stderr, err)
	return cli.ExitUsage
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: fox-openapi <generate|check|serve|version> [flags]")
}

type repeatedFlag struct {
	values []string
	set    bool
}

func (f *repeatedFlag) String() string { return strings.Join(f.values, ",") }

func (f *repeatedFlag) Set(value string) error {
	f.set = true
	f.values = append(f.values, value)
	return nil
}
