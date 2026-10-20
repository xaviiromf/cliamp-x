package player

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ProcessInfo describes how the system schedules this process, for the
// Android diagnostics: its cgroups, the CPUs it may run on, and the policy of
// the OpenSL ES callback thread ("AudioTrack"), which Android runs real-time
// (rt) only for a FAST track. Parts it cannot read are left out, so it is ""
// off Linux.
func ProcessInfo() string {
	var parts []string
	if b, err := os.ReadFile("/proc/self/cgroup"); err == nil {
		parts = append(parts, "cgroup="+strings.Join(strings.Fields(string(b)), ";"))
	}
	if b, err := os.ReadFile("/proc/self/status"); err == nil {
		for line := range strings.SplitSeq(string(b), "\n") {
			if v, ok := strings.CutPrefix(line, "Cpus_allowed_list:"); ok {
				parts = append(parts, "cpus="+strings.TrimSpace(v))
			}
		}
	}
	tasks, _ := filepath.Glob("/proc/self/task/*")
	for _, t := range tasks {
		if comm, _ := os.ReadFile(filepath.Join(t, "comm")); strings.TrimSpace(string(comm)) != "AudioTrack" {
			continue
		}
		stat, _ := os.ReadFile(filepath.Join(t, "stat"))
		if policy, ok := statPolicy(string(stat)); ok {
			kind := "normal"
			if policy == 1 || policy == 2 { // SCHED_FIFO, SCHED_RR
				kind = "rt"
			}
			parts = append(parts, "audio_thread="+kind)
			break
		}
	}
	return strings.Join(parts, " ")
}

// statPolicy reads the scheduling policy, field 41, from a /proc stat line.
// The command name in field 2 may hold spaces and parentheses, so the fields
// are counted from its last ')'.
func statPolicy(stat string) (int, bool) {
	i := strings.LastIndexByte(stat, ')')
	if i < 0 {
		return 0, false
	}
	f := strings.Fields(stat[i+1:])
	if len(f) < 39 {
		return 0, false
	}
	n, err := strconv.Atoi(f[38])
	return n, err == nil
}
