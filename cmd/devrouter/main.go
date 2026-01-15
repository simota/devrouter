package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"devrouter/internal/devrouter"
)

// Build-time variables (injected via ldflags)
var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

func main() {
	os.Exit(run())
}

func run() int {
	if len(os.Args) < 2 {
		usage()
		return 1
	}
	cmd := os.Args[1]
	switch cmd {
	case "-h", "--help", "help":
		usage()
		return 0
	case "-v", "--version", "version":
		fmt.Printf("devrouter %s\n", version)
		fmt.Printf("  commit:  %s\n", commit)
		fmt.Printf("  built:   %s\n", buildDate)
		fmt.Printf("  go:      %s\n", runtime.Version())
		return 0
	case "up":
		return runUp(os.Args[2:])
	case "down":
		return runDown(os.Args[2:])
	case "ls":
		return runLs()
	case "logs":
		return runLogs(os.Args[2:])
	case "open":
		return runOpen(os.Args[2:])
	case "init":
		return runInit(os.Args[2:])
	case "exec":
		return runExec(os.Args[2:])
	case "doctor":
		return runDoctor(os.Args[2:])
	case "daemon":
		return runDaemon(os.Args[2:])
	case "ui":
		return runUI(os.Args[2:])
	case "history":
		return runHistory(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		usage()
		return 1
	}
}

func usage() {
	fmt.Println("devrouter - ローカル開発ルータ")
	fmt.Println("")
	fmt.Println("Usage:")
	fmt.Println("  devrouter up [path]")
	fmt.Println("  devrouter down [path|stack]")
	fmt.Println("  devrouter ls")
	fmt.Println("  devrouter logs <stack>[/<service>] [-f]")
	fmt.Println("  devrouter open <stack>/<service>")
	fmt.Println("  devrouter init [path]")
	fmt.Println("  devrouter exec <stack>/<service> [--] <cmd>")
	fmt.Println("  devrouter doctor [path]")
	fmt.Println("  devrouter daemon up|down")
	fmt.Println("  devrouter ui [--addr :19847] [--no-open]")
	fmt.Println("  devrouter history <stack> [--stats] [--insights]")
	fmt.Println("")
	fmt.Println("Options (up):")
	fmt.Println("  --config <path>   devrouter.yaml を指定")
	fmt.Println("  --compose <path>  composeファイルを指定 (devrouter.yamlのcomposeを上書き)")
	fmt.Println("  --domain <domain> ホストドメインを上書き")
	fmt.Println("  --no-daemon       Traefikの自動起動を無効化")
	fmt.Println("  --build           イメージを再ビルドして起動")
	fmt.Println("")
	fmt.Println("Options (init):")
	fmt.Println("  --compose <path>  docker-compose.yaml のパス")
	fmt.Println("  --output <path>   出力先 (default: devrouter.yaml)")
	fmt.Println("  --stack <name>    stack名を指定")
	fmt.Println("  --domain <domain> ドメインを指定")
	fmt.Println("  --force           既存ファイルを上書き")
	fmt.Println("")
	fmt.Println("Options (exec):")
	fmt.Println("  --user <name>     実行ユーザーを指定")
	fmt.Println("  --workdir <path>  実行ディレクトリを指定")
	fmt.Println("")
	fmt.Println("Options (doctor):")
	fmt.Println("  --config <path>   devrouter.yaml を指定")
	fmt.Println("  --compose <path>  composeファイルを指定")
	fmt.Println("")
	fmt.Println("Options (daemon up):")
	fmt.Println("  --config <path>   devrouter.yaml を指定")
	fmt.Println("")
	fmt.Println("Options (ui):")
	fmt.Println("  --addr <addr>     リッスンアドレス (default: :19847)")
	fmt.Println("  --no-open         ブラウザを自動で開かない")
	fmt.Println("")
	fmt.Println("Options (history):")
	fmt.Println("  --stats           Stats履歴を表示")
	fmt.Println("  --insights        インサイトを表示")
	fmt.Println("  --range <range>   Stats範囲 (1h, 6h, 24h) (default: 1h)")
}

func runUp(args []string) int {
	fs := flag.NewFlagSet("up", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	configPath := fs.String("config", "", "config path")
	composePath := fs.String("compose", "", "compose path")
	domain := fs.String("domain", "", "domain")
	noDaemon := fs.Bool("no-daemon", false, "disable daemon")
	build := fs.Bool("build", false, "rebuild images")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	path := "."
	if fs.NArg() > 0 {
		path = fs.Arg(0)
	}
	record, warnings, err := devrouter.Up(path, devrouter.UpOptions{
		ConfigPath:  *configPath,
		ComposePath: *composePath,
		Domain:      *domain,
		AutoDaemon:  !*noDaemon,
		Build:       *build,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	for _, warning := range warnings {
		fmt.Fprintln(os.Stderr, "warning:", warning)
	}
	fmt.Printf("stack: %s (%s)\n", record.Name, record.ID)
	for _, svc := range record.Services {
		fmt.Printf("- %s: %s (%s)\n", svc.Name, svc.URL, svc.WorkspacePath)
	}
	return 0
}

func runDown(args []string) int {
	fs := flag.NewFlagSet("down", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	target := ""
	if fs.NArg() > 0 {
		target = fs.Arg(0)
	}
	record, err := devrouter.Down(devrouter.DownOptions{Target: target})
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	fmt.Printf("stopped: %s (%s)\n", record.Name, record.ID)
	return 0
}

func runLs() int {
	reg, err := devrouter.List()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	if len(reg.Stacks) == 0 {
		fmt.Println("no stacks")
		return 0
	}
	fmt.Println("stack\tservice\tsource\turl\tworkspace")
	for _, stack := range reg.Stacks {
		source := stack.Source
		if source == "" {
			source = "unknown"
		}
		for _, svc := range stack.Services {
			fmt.Printf("%s\t%s\t%s\t%s\t%s\n", stack.Name, svc.Name, source, svc.URL, svc.WorkspacePath)
		}
	}
	return 0
}

func runLogs(args []string) int {
	target, follow, err := parseLogsArgs(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	if err := devrouter.Logs(target, devrouter.LogsOptions{Follow: follow}); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	return 0
}

func parseLogsArgs(args []string) (string, bool, error) {
	var target string
	follow := false
	for _, arg := range args {
		switch {
		case arg == "-f":
			follow = true
		case strings.HasPrefix(arg, "-"):
			return "", follow, fmt.Errorf("unknown flag: %s", arg)
		case target == "":
			target = arg
		default:
			return "", follow, fmt.Errorf("unexpected argument: %s", arg)
		}
	}
	if target == "" {
		return "", follow, fmt.Errorf("target is required")
	}
	return target, follow, nil
}

func runOpen(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "error: target is required")
		return 1
	}
	target := args[0]
	if strings.TrimSpace(target) == "" {
		fmt.Fprintln(os.Stderr, "error: target is required")
		return 1
	}
	url, err := devrouter.Open(target)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	if err := devrouter.OpenURL(url); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	fmt.Printf("opened: %s\n", url)
	return 0
}

func runExec(args []string) int {
	target, command, opts, err := parseExecArgs(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	if err := devrouter.Exec(target, command, opts); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	return 0
}

func runInit(args []string) int {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	composePath := fs.String("compose", "", "compose path")
	outputPath := fs.String("output", "", "output path")
	stackName := fs.String("stack", "", "stack name")
	domain := fs.String("domain", "", "domain")
	force := fs.Bool("force", false, "overwrite")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	path := "."
	if fs.NArg() > 0 {
		path = fs.Arg(0)
	}
	result, err := devrouter.InitConfig(devrouter.InitOptions{
		RepoPath:    path,
		ComposePath: *composePath,
		OutputPath:  *outputPath,
		Stack:       *stackName,
		Domain:      *domain,
		Force:       *force,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	fmt.Printf("generated: %s\n", result.OutputPath)
	for _, warning := range result.Warnings {
		fmt.Fprintf(os.Stderr, "warning: %s\n", warning)
	}
	return 0
}

func runDoctor(args []string) int {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	configPath := fs.String("config", "", "config path")
	composePath := fs.String("compose", "", "compose path")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	path := "."
	if fs.NArg() > 0 {
		path = fs.Arg(0)
	}
	report, err := devrouter.Doctor(path, devrouter.DoctorOptions{
		ConfigPath:  *configPath,
		ComposePath: *composePath,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	printDoctorReport(report)
	if report.HasErrors() {
		return 1
	}
	return 0
}

func runDaemon(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "error: daemon subcommand is required (up|down)")
		return 1
	}
	sub := args[0]
	switch sub {
	case "up":
		fs := flag.NewFlagSet("daemon up", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		configPath := fs.String("config", "", "config path")
		if err := fs.Parse(args[1:]); err != nil {
			return 2
		}
		cfg := devrouter.DefaultConfig()
		if *configPath != "" {
			absConfigPath, err := filepath.Abs(*configPath)
			if err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				return 1
			}
			configDir := filepath.Dir(absConfigPath)
			loaded, _, err := devrouter.LoadConfig(configDir, absConfigPath)
			if err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
				return 1
			}
			cfg = loaded
			cfg.TLS = resolveTLSRelativePaths(configDir, cfg.TLS)
		}
		if err := devrouter.DaemonUp(devrouter.DaemonOptions{TLS: cfg.TLS}); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			return 1
		}
		fmt.Println("daemon started")
		return 0
	case "down":
		if err := devrouter.DaemonDown(); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			return 1
		}
		fmt.Println("daemon stopped")
		return 0
	default:
		fmt.Fprintf(os.Stderr, "error: unknown daemon command: %s\n", sub)
		return 1
	}
}

func resolveTLSRelativePaths(baseDir string, cfg devrouter.TLSConfig) devrouter.TLSConfig {
	if cfg.CertFile != "" && !filepath.IsAbs(cfg.CertFile) && !strings.HasPrefix(cfg.CertFile, "~") {
		cfg.CertFile = filepath.Join(baseDir, cfg.CertFile)
	}
	if cfg.KeyFile != "" && !filepath.IsAbs(cfg.KeyFile) && !strings.HasPrefix(cfg.KeyFile, "~") {
		cfg.KeyFile = filepath.Join(baseDir, cfg.KeyFile)
	}
	return cfg
}

func printDoctorReport(report devrouter.DoctorReport) {
	for _, check := range report.Checks {
		label := strings.ToUpper(string(check.Level))
		if check.Name != "" {
			fmt.Printf("[%s] %s: %s\n", label, check.Name, check.Message)
		} else {
			fmt.Printf("[%s] %s\n", label, check.Message)
		}
		if check.Hint != "" {
			fmt.Printf("  hint: %s\n", check.Hint)
		}
	}
}

func parseExecArgs(args []string) (string, []string, devrouter.ExecOptions, error) {
	if len(args) == 0 {
		return "", nil, devrouter.ExecOptions{}, fmt.Errorf("target is required")
	}
	var opts devrouter.ExecOptions
	split := -1
	for i, arg := range args {
		if arg == "--" {
			split = i
			break
		}
	}
	if split >= 0 {
		target, err := parseExecHead(args[:split], &opts)
		if err != nil {
			return "", nil, opts, err
		}
		command := args[split+1:]
		if len(command) == 0 {
			return "", nil, opts, fmt.Errorf("command is required after --")
		}
		return target, command, opts, nil
	}

	target, command, err := parseExecWithoutSeparator(args, &opts)
	if err != nil {
		return "", nil, opts, err
	}
	if len(command) == 0 {
		return "", nil, opts, fmt.Errorf("command is required")
	}
	return target, command, opts, nil
}

func parseExecHead(args []string, opts *devrouter.ExecOptions) (string, error) {
	var target string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--user" || arg == "-u":
			if i+1 >= len(args) {
				return "", fmt.Errorf("missing value for %s", arg)
			}
			opts.User = args[i+1]
			i++
		case strings.HasPrefix(arg, "--user="):
			opts.User = strings.TrimPrefix(arg, "--user=")
		case arg == "--workdir" || arg == "-w":
			if i+1 >= len(args) {
				return "", fmt.Errorf("missing value for %s", arg)
			}
			opts.Workdir = args[i+1]
			i++
		case strings.HasPrefix(arg, "--workdir="):
			opts.Workdir = strings.TrimPrefix(arg, "--workdir=")
		case strings.HasPrefix(arg, "-"):
			return "", fmt.Errorf("unknown flag: %s", arg)
		case target == "":
			target = arg
		default:
			return "", fmt.Errorf("unexpected argument: %s", arg)
		}
	}
	if target == "" {
		return "", fmt.Errorf("target is required")
	}
	return target, nil
}

func parseExecWithoutSeparator(args []string, opts *devrouter.ExecOptions) (string, []string, error) {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--user" || arg == "-u":
			if i+1 >= len(args) {
				return "", nil, fmt.Errorf("missing value for %s", arg)
			}
			opts.User = args[i+1]
			i++
		case strings.HasPrefix(arg, "--user="):
			opts.User = strings.TrimPrefix(arg, "--user=")
		case arg == "--workdir" || arg == "-w":
			if i+1 >= len(args) {
				return "", nil, fmt.Errorf("missing value for %s", arg)
			}
			opts.Workdir = args[i+1]
			i++
		case strings.HasPrefix(arg, "--workdir="):
			opts.Workdir = strings.TrimPrefix(arg, "--workdir=")
		case strings.HasPrefix(arg, "-"):
			return "", nil, fmt.Errorf("unknown flag: %s", arg)
		default:
			target := arg
			command := args[i+1:]
			return target, command, nil
		}
	}
	return "", nil, fmt.Errorf("target is required")
}

func runUI(args []string) int {
	fs := flag.NewFlagSet("ui", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	addr := fs.String("addr", ":19847", "listen address")
	noOpen := fs.Bool("no-open", false, "do not open browser")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	server := devrouter.NewServer(devrouter.ServerConfig{
		Addr: *addr,
	})

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Start()
	}()

	url := "http://localhost" + *addr
	fmt.Printf("devrouter UI: %s\n", url)

	if !*noOpen {
		if err := devrouter.OpenURL(url); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not open browser: %v\n", err)
		}
	}

	fmt.Println("Press Ctrl+C to stop")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-errCh:
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	case <-sigCh:
		fmt.Println("\nshutting down...")
		ctx, cancel := context.WithTimeout(context.Background(), 5*1e9)
		defer cancel()
		server.Shutdown(ctx)
	}

	return 0
}

func runHistory(args []string) int {
	fs := flag.NewFlagSet("history", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	showStats := fs.Bool("stats", false, "show stats history")
	showInsights := fs.Bool("insights", false, "show insights")
	rangeStr := fs.String("range", "1h", "stats range (1h, 6h, 24h)")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if fs.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "error: stack name or ID is required")
		return 1
	}
	stackID := fs.Arg(0)

	// Default: show events
	if !*showStats && !*showInsights {
		return showEventHistory(stackID)
	}

	if *showStats {
		return showStatsHistory(stackID, *rangeStr)
	}

	if *showInsights {
		return showStackInsights(stackID)
	}

	return 0
}

func showEventHistory(stackID string) int {
	since := time.Now().UTC().Add(-24 * time.Hour)
	events, err := devrouter.LoadEvents(stackID, since)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}

	if len(events) == 0 {
		fmt.Println("no events in the last 24 hours")
		return 0
	}

	fmt.Printf("Events for %s (last 24h):\n\n", stackID)
	for _, e := range events {
		ts, _ := time.Parse(time.RFC3339, e.Timestamp)
		timeStr := ts.Local().Format("2006-01-02 15:04:05")

		switch e.Type {
		case devrouter.EventTypeUp:
			services := strings.Join(e.Services, ", ")
			if services == "" {
				services = "-"
			}
			fmt.Printf("[%s] UP    %s (services: %s)\n", timeStr, e.Stack, services)
		case devrouter.EventTypeDown:
			duration := e.Duration
			if duration == "" {
				duration = "-"
			}
			fmt.Printf("[%s] DOWN  %s (duration: %s)\n", timeStr, e.Stack, duration)
		case devrouter.EventTypeRestart:
			trigger := e.Trigger
			if trigger == "" {
				trigger = "manual"
			}
			fmt.Printf("[%s] RESTART %s/%s (trigger: %s)\n", timeStr, e.Stack, e.Service, trigger)
		}
	}

	return 0
}

func showStatsHistory(stackID, rangeStr string) int {
	var duration time.Duration
	switch rangeStr {
	case "1h":
		duration = 1 * time.Hour
	case "6h":
		duration = 6 * time.Hour
	case "24h":
		duration = 24 * time.Hour
	default:
		fmt.Fprintln(os.Stderr, "error: invalid range (use 1h, 6h, or 24h)")
		return 1
	}

	now := time.Now().UTC()
	since := now.Add(-duration)

	stats, err := devrouter.LoadStats(stackID, since, now)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}

	if len(stats) == 0 {
		fmt.Printf("no stats for %s in the last %s\n", stackID, rangeStr)
		return 0
	}

	fmt.Printf("Stats for %s (last %s):\n\n", stackID, rangeStr)
	fmt.Println("TIMESTAMP\t\t\tSERVICE\t\tCPU\tMEM")
	fmt.Println("─────────────────────────────────────────────────────────────")

	for _, s := range stats {
		ts, _ := time.Parse(time.RFC3339, s.Timestamp)
		timeStr := ts.Local().Format("2006-01-02 15:04:05")
		memMB := float64(s.Mem) / (1024 * 1024)
		fmt.Printf("%s\t%-12s\t%.1f%%\t%.1fMB\n", timeStr, s.Service, s.CPU, memMB)
	}

	return 0
}

func showStackInsights(stackID string) int {
	insights, err := devrouter.ComputeInsights(stackID)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}

	fmt.Printf("Insights for %s:\n\n", stackID)
	fmt.Printf("  Uptime:         %s\n", insights.Uptime)
	fmt.Printf("  Restarts today: %d\n", insights.RestartCount)

	if len(insights.RestartsByService) > 0 {
		fmt.Printf("    By service:\n")
		for svc, count := range insights.RestartsByService {
			fmt.Printf("      - %s: %d\n", svc, count)
		}
	}

	if insights.PeakMemory > 0 {
		peakMB := float64(insights.PeakMemory) / (1024 * 1024)
		fmt.Printf("  Peak memory:    %.1fMB (%s @ %s)\n", peakMB, insights.PeakMemorySvc, insights.PeakMemoryAt)
	}

	if insights.AvgCPU > 0 {
		fmt.Printf("  Avg CPU:        %.1f%%\n", insights.AvgCPU)
	}

	return 0
}
