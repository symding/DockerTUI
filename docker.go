package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	dockerclient "github.com/docker/docker/client"
)

type logSession struct {
	id          int
	lines       <-chan string
	cancel      context.CancelFunc
	stripPrefix bool
}

type inspectLoadedMsg struct {
	from    section
	title   string
	content string
	err     error
}

type logLineMsg struct {
	id   int
	line string
}

type logClosedMsg struct {
	id int
}

func dockerAPIClient() (*dockerclient.Client, error) {
	return dockerclient.NewClientWithOpts(dockerclient.FromEnv, dockerclient.WithAPIVersionNegotiation())
}

func dockerOutput(args ...string) (string, error) {
	cmd := exec.Command("docker", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("docker %s: %s", strings.Join(args, " "), msg)
	}
	return string(out), nil
}

func startLog(id int, args []string) *logSession {
	ctx, cancel := context.WithCancel(context.Background())
	lines := make(chan string, 200)
	go streamDockerLogs(ctx, lines, args...)
	return &logSession{id: id, lines: lines, cancel: cancel}
}

func streamDockerLogs(ctx context.Context, lines chan<- string, args ...string) {
	defer close(lines)

	cmd := exec.CommandContext(ctx, "docker", args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		lines <- err.Error()
		return
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		lines <- err.Error()
		return
	}
	if err := cmd.Start(); err != nil {
		lines <- err.Error()
		return
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go scanLines(stdout, lines, &wg)
	go scanLines(stderr, lines, &wg)
	wg.Wait()

	if err := cmd.Wait(); err != nil && ctx.Err() == nil {
		lines <- err.Error()
	}
}

func scanLines(r io.Reader, lines chan<- string, wg *sync.WaitGroup) {
	defer wg.Done()
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		lines <- scanner.Text()
	}
}

func waitLogLine(id int, lines <-chan string) tea.Cmd {
	return func() tea.Msg {
		line, ok := <-lines
		if !ok {
			return logClosedMsg{id: id}
		}
		return logLineMsg{id: id, line: line}
	}
}

func stripServiceLogPrefix(line string) string {
	if i := strings.Index(line, " | "); i >= 0 {
		return line[i+3:]
	}
	return line
}

func splitLines(out string) []string {
	out = strings.TrimSpace(out)
	if out == "" {
		return nil
	}
	return strings.Split(out, "\n")
}

func formatEnv(env []string) string {
	if len(env) == 0 {
		return "  (none)\n"
	}
	var b strings.Builder
	for _, item := range env {
		fmt.Fprintf(&b, "  %s\n", item)
	}
	return b.String()
}

func formatDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	d = d.Round(time.Minute)
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60
	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}
