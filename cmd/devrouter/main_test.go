package main

import (
	"reflect"
	"testing"

	"devrouter/internal/devrouter"
)

func TestParseExecArgs(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantTarget  string
		wantCommand []string
		wantOpts    devrouter.ExecOptions
		wantErr     string
	}{
		{
			name:        "with separator and flags",
			args:        []string{"--user", "root", "--workdir", "/app", "stack/service", "--", "npm", "start"},
			wantTarget:  "stack/service",
			wantCommand: []string{"npm", "start"},
			wantOpts: devrouter.ExecOptions{
				User:    "root",
				Workdir: "/app",
			},
		},
		{
			name:        "without separator uses remaining as command",
			args:        []string{"-u", "alice", "-w", "/srv", "stack/service", "bash", "-lc", "echo"},
			wantTarget:  "stack/service",
			wantCommand: []string{"bash", "-lc", "echo"},
			wantOpts: devrouter.ExecOptions{
				User:    "alice",
				Workdir: "/srv",
			},
		},
		{
			name:        "equals style flags",
			args:        []string{"stack/service", "--user=alice", "--workdir=/home/alice", "--", "sh", "-c", "pwd"},
			wantTarget:  "stack/service",
			wantCommand: []string{"sh", "-c", "pwd"},
			wantOpts:    devrouter.ExecOptions{User: "alice", Workdir: "/home/alice"},
		},
		{
			name:    "missing args returns target error",
			args:    []string{},
			wantErr: "target is required",
		},
		{
			name:    "missing command after separator",
			args:    []string{"stack/service", "--"},
			wantErr: "command is required after --",
		},
		{
			name:    "missing command without separator",
			args:    []string{"stack/service"},
			wantErr: "command is required",
		},
		{
			name:    "unknown flag surfaces error",
			args:    []string{"--unknown", "stack/service"},
			wantErr: "unknown flag: --unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target, command, opts, err := parseExecArgs(tt.args)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error %q, got nil", tt.wantErr)
				}
				if err.Error() != tt.wantErr {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if target != tt.wantTarget {
				t.Fatalf("target mismatch: got %q want %q", target, tt.wantTarget)
			}
			if !reflect.DeepEqual(command, tt.wantCommand) {
				t.Fatalf("command mismatch: got %v want %v", command, tt.wantCommand)
			}
			if opts != tt.wantOpts {
				t.Fatalf("opts mismatch: got %+v want %+v", opts, tt.wantOpts)
			}
		})
	}
}
