package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/versions"
	dockerclient "github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
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

func startContainerLog(id int, containerID string) *logSession {
	ctx, cancel := context.WithCancel(context.Background())
	lines := make(chan string, 200)
	go streamContainerLogs(ctx, lines, containerID)
	return &logSession{id: id, lines: lines, cancel: cancel}
}

func startServiceLog(id int, taskID string, tty bool) *logSession {
	ctx, cancel := context.WithCancel(context.Background())
	lines := make(chan string, 200)
	go streamServiceLogs(ctx, lines, taskID, tty)
	return &logSession{id: id, lines: lines, cancel: cancel}
}

func streamContainerLogs(ctx context.Context, lines chan<- string, containerID string) {
	defer close(lines)

	cli, err := dockerAPIClient()
	if err != nil {
		lines <- err.Error()
		return
	}
	defer cli.Close()

	cli.NegotiateAPIVersion(ctx)
	inspect, err := cli.ContainerInspect(ctx, containerID)
	if err != nil {
		lines <- err.Error()
		return
	}

	reader, err := cli.ContainerLogs(ctx, containerID, containertypes.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Tail:       "100",
	})
	if err != nil {
		lines <- err.Error()
		return
	}
	defer reader.Close()

	tty := inspect.Config != nil && inspect.Config.Tty
	scanLogReader(reader, lines, !tty && versions.GreaterThanOrEqualTo(cli.ClientVersion(), "1.42"))
}

func streamServiceLogs(ctx context.Context, lines chan<- string, taskID string, tty bool) {
	defer close(lines)

	cli, err := dockerAPIClient()
	if err != nil {
		lines <- err.Error()
		return
	}
	defer cli.Close()

	cli.NegotiateAPIVersion(ctx)
	reader, err := cli.TaskLogs(ctx, taskID, containertypes.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
		Tail:       "100",
	})
	if err != nil {
		lines <- err.Error()
		return
	}
	defer reader.Close()

	scanLogReader(reader, lines, !tty && versions.GreaterThanOrEqualTo(cli.ClientVersion(), "1.42"))
}

func scanLogReader(r io.Reader, lines chan<- string, multiplexed bool) {
	if multiplexed {
		src := r
		pr, pw := io.Pipe()
		go func() {
			_, err := stdcopy.StdCopy(pw, pw, src)
			_ = pw.CloseWithError(err)
		}()
		r = pr
	}
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		lines <- scanner.Text()
	}
	if err := scanner.Err(); err != nil {
		lines <- err.Error()
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
