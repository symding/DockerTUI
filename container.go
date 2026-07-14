package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"dockertui/internal/component"

	btable "github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	containertypes "github.com/docker/docker/api/types/container"
	dockerclient "github.com/docker/docker/client"
)

type containerAction int

const (
	containerActionInspect containerAction = iota
	containerActionLogs
	containerActionRestart
	containerActionStop
	containerActionKill
	containerActionStart
)

type containerItem struct {
	ID      string
	Name    string
	Image   string
	Command string
	Created string
	Status  string
	Ports   string
}

type containersLoadedMsg struct {
	items []containerItem
	err   error
}

type containerUpdatedMsg struct {
	err error
}

type statsTickMsg struct {
	id int
}

type statsLoadedMsg struct {
	id     int
	cpu    string
	memory string
	err    error
}

func (m *model) setContainerActionTable() {
	setActionRows(&m.action, []btable.Row{
		{"[I] inspect"},
		{"[L] logs"},
		{"[R] restart"},
		{"[S] stop"},
		{"[K] kill"},
		{"[T] start"},
	})
}

func (m model) openContainerActionShortcut(key string) (tea.Model, tea.Cmd, bool) {
	switch key {
	case "i":
		m.action.SetCursor(int(containerActionInspect))
	case "l":
		m.action.SetCursor(int(containerActionLogs))
	case "r":
		m.action.SetCursor(int(containerActionRestart))
	case "s":
		m.action.SetCursor(int(containerActionStop))
	case "k":
		m.action.SetCursor(int(containerActionKill))
	case "t":
		m.action.SetCursor(int(containerActionStart))
	default:
		return m, nil, false
	}
	updated, cmd := m.openContainerAction()
	return updated, cmd, true
}

func (m *model) setContainerTable() {
	rows := make([]btable.Row, 0, len(m.containers))
	m.filteredContainers = m.filteredContainers[:0]
	for i, c := range m.containers {
		if !matchesFilter(m.containerFilter, c.ID, c.Name, c.Image, c.Command, c.Created, c.Status, c.Ports) {
			continue
		}
		m.filteredContainers = append(m.filteredContainers, i)
		rows = append(rows, btable.Row{c.ID, c.Name, c.Image, c.Command, c.Created, c.Status, c.Ports})
	}
	component.SetTableRows(&m.table, containerColumns(m.rightWidth()), rows, filteredCursor(m.filteredContainers, m.selectedContainer))
	m.resizeComponents()
	m.applyFocus()
}

func (m model) openContainerAction() (tea.Model, tea.Cmd) {
	switch containerAction(m.action.Cursor()) {
	case containerActionLogs:
		return m.openContainerLogs()
	case containerActionInspect:
		return m.openContainerInspect()
	case containerActionStop:
		return m.runContainerAction("stop")
	case containerActionKill:
		return m.runContainerAction("kill")
	case containerActionStart:
		return m.runContainerAction("start")
	default:
		return m.runContainerAction("restart")
	}
}

func (m model) runContainerAction(action string) (tea.Model, tea.Cmd) {
	m.actionOpen = false
	m.status = fmt.Sprintf("Running container %s", action)
	return m, updateContainer(action, m.activeContainer.ID)
}

func (m model) openContainerLogs() (tea.Model, tea.Cmd) {
	m.actionOpen = false
	m.stopLog()
	m.taskLogOpen = true
	m.logs = nil
	m.logID++
	m.logTitle = fmt.Sprintf("Container Logs: %s  Esc close", m.activeContainer.Name)
	m.logStatus = fmt.Sprintf("%d containers", len(m.containers))
	m.resizeComponents()
	m.updateLogView()
	s := startContainerLog(m.logID, m.activeContainer.ID)
	m.logSession = s
	m.status = "Following container logs"
	return m, waitLogLine(s.id, s.lines)
}

func (m model) openContainerInspect() (tea.Model, tea.Cmd) {
	m.actionOpen = false
	m.status = "Loading container inspect"
	return m, loadContainerInspect(m.activeContainer.ID)
}

func loadContainers() tea.Cmd {
	return func() tea.Msg {
		cli, err := dockerAPIClient()
		if err != nil {
			return containersLoadedMsg{err: err}
		}
		defer cli.Close()

		containers, err := cli.ContainerList(context.Background(), containertypes.ListOptions{All: true})
		if err != nil {
			return containersLoadedMsg{err: err}
		}
		items := make([]containerItem, 0, len(containers))
		for _, c := range containers {
			items = append(items, containerItem{
				ID:      shortID(c.ID),
				Name:    formatContainerNames(c.Names),
				Image:   c.Image,
				Command: c.Command,
				Created: formatCreated(c.Created),
				Status:  c.Status,
				Ports:   formatContainerPorts(c.Ports),
			})
		}
		return containersLoadedMsg{items: items}
	}
}

func loadContainerInspect(containerID string) tea.Cmd {
	return func() tea.Msg {
		cli, err := dockerAPIClient()
		if err != nil {
			return inspectLoadedMsg{err: err}
		}
		defer cli.Close()

		ctx := context.Background()
		info, err := cli.ContainerInspect(ctx, containerID)
		if err != nil {
			return inspectLoadedMsg{err: err}
		}
		stats, ok := loadContainerStats(ctx, cli, containerID)
		name := strings.TrimPrefix(info.Name, "/")
		return inspectLoadedMsg{
			from:    sectionContainers,
			title:   fmt.Sprintf("Container Inspect: %s", name),
			content: formatContainerInspect(info, stats, ok),
		}
	}
}

func loadContainerStats(ctx context.Context, cli *dockerclient.Client, containerID string) (containertypes.StatsResponse, bool) {
	reader, err := cli.ContainerStatsOneShot(ctx, containerID)
	if err != nil {
		return containertypes.StatsResponse{}, false
	}
	defer reader.Body.Close()

	var stats containertypes.StatsResponse
	if err := json.NewDecoder(reader.Body).Decode(&stats); err != nil {
		return containertypes.StatsResponse{}, false
	}
	return stats, true
}

func updateContainer(action, containerID string) tea.Cmd {
	return func() tea.Msg {
		cli, err := dockerAPIClient()
		if err != nil {
			return containerUpdatedMsg{err: err}
		}
		defer cli.Close()

		ctx := context.Background()
		switch action {
		case "start":
			err = cli.ContainerStart(ctx, containerID, containertypes.StartOptions{})
		case "stop":
			err = cli.ContainerStop(ctx, containerID, containertypes.StopOptions{})
		case "kill":
			err = cli.ContainerKill(ctx, containerID, "")
		default:
			err = cli.ContainerRestart(ctx, containerID, containertypes.StopOptions{})
		}
		return containerUpdatedMsg{err: err}
	}
}

func shortID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

func formatContainerNames(names []string) string {
	trimmed := make([]string, 0, len(names))
	for _, name := range names {
		trimmed = append(trimmed, strings.TrimPrefix(name, "/"))
	}
	return strings.Join(trimmed, ", ")
}

func formatCreated(created int64) string {
	if created == 0 {
		return "-"
	}
	return formatDuration(time.Since(time.Unix(created, 0))) + " ago"
}

func formatContainerPorts(ports []containertypes.Port) string {
	values := make([]string, 0, len(ports))
	for _, port := range ports {
		target := fmt.Sprintf("%d/%s", port.PrivatePort, port.Type)
		if port.PublicPort == 0 {
			values = append(values, target)
			continue
		}
		host := fmt.Sprintf("%d", port.PublicPort)
		if port.IP != "" {
			host = port.IP + ":" + host
		}
		values = append(values, fmt.Sprintf("%s->%s", host, target))
	}
	sort.Strings(values)
	return strings.Join(values, ", ")
}

func formatContainerInspect(info containertypes.InspectResponse, stats containertypes.StatsResponse, hasStats bool) string {
	name := strings.TrimPrefix(info.Name, "/")
	image := "-"
	env := []string(nil)
	if info.Config != nil {
		image = info.Config.Image
		env = info.Config.Env
	}
	if image == "" {
		image = info.Image
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Name: %s\n", name)
	fmt.Fprintf(&b, "Image: %s\n", image)
	fmt.Fprintf(&b, "Runtime: %s\n\n", containerRuntime(info.State))
	b.WriteString("Resource Chart\n")
	if hasStats {
		cpu := containerCPUPercent(stats)
		mem := memoryPercent(stats)
		fmt.Fprintf(&b, "%s\n", barLine("CPU", cpu, fmt.Sprintf("%.2f%%", cpu)))
		fmt.Fprintf(&b, "%s\n", barLine("Memory", mem, fmt.Sprintf("%s / %s (%.2f%%)", formatBytes(stats.MemoryStats.Usage), formatBytes(stats.MemoryStats.Limit), mem)))
	} else {
		b.WriteString("CPU    [------------------------------] unavailable\n")
		b.WriteString("Memory [------------------------------] unavailable\n")
	}
	b.WriteString("\nEnvironment\n")
	b.WriteString(formatEnv(env))
	return b.String()
}

func containerRuntime(state *containertypes.State) string {
	if state == nil {
		return "-"
	}
	start, ok := parseDockerTime(state.StartedAt)
	if !ok {
		return string(state.Status)
	}
	if state.Running {
		return formatDuration(time.Since(start))
	}
	finished, ok := parseDockerTime(state.FinishedAt)
	if ok && finished.After(start) {
		return formatDuration(finished.Sub(start))
	}
	return string(state.Status)
}

func parseDockerTime(value string) (time.Time, bool) {
	t, err := time.Parse(time.RFC3339Nano, value)
	if err != nil || t.IsZero() || t.Year() <= 1 {
		return time.Time{}, false
	}
	return t, true
}

func containerCPUPercent(stats containertypes.StatsResponse) float64 {
	cpuDelta := float64(stats.CPUStats.CPUUsage.TotalUsage - stats.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(stats.CPUStats.SystemUsage - stats.PreCPUStats.SystemUsage)
	onlineCPUs := float64(stats.CPUStats.OnlineCPUs)
	if onlineCPUs == 0 {
		onlineCPUs = float64(len(stats.CPUStats.CPUUsage.PercpuUsage))
	}
	if cpuDelta <= 0 || systemDelta <= 0 || onlineCPUs <= 0 {
		return 0
	}
	return cpuDelta / systemDelta * onlineCPUs * 100
}

func memoryPercent(stats containertypes.StatsResponse) float64 {
	if stats.MemoryStats.Limit == 0 {
		return 0
	}
	return float64(stats.MemoryStats.Usage) / float64(stats.MemoryStats.Limit) * 100
}

func barLine(label string, percent float64, suffix string) string {
	width := 30
	filled := int(percent / 100 * float64(width))
	filled = min(max(filled, 0), width)
	return fmt.Sprintf("%-6s [%s%s] %s", label, strings.Repeat("#", filled), strings.Repeat("-", width-filled), suffix)
}

func formatBytes(value uint64) string {
	units := []string{"B", "KiB", "MiB", "GiB", "TiB"}
	size := float64(value)
	unit := 0
	for size >= 1024 && unit < len(units)-1 {
		size /= 1024
		unit++
	}
	if unit == 0 {
		return fmt.Sprintf("%d%s", value, units[unit])
	}
	return fmt.Sprintf("%.1f%s", size, units[unit])
}

func loadStats(id int, containerID string) tea.Cmd {
	return func() tea.Msg {
		cli, err := dockerAPIClient()
		if err != nil {
			return statsLoadedMsg{id: id, err: err}
		}
		defer cli.Close()

		stats, ok := loadContainerStats(context.Background(), cli, containerID)
		if !ok {
			return statsLoadedMsg{id: id, cpu: "-", memory: "-"}
		}
		return statsLoadedMsg{
			id:     id,
			cpu:    fmt.Sprintf("%.2f%%", containerCPUPercent(stats)),
			memory: fmt.Sprintf("%s / %s", formatBytes(stats.MemoryStats.Usage), formatBytes(stats.MemoryStats.Limit)),
		}
	}
}

func tickStats(id int) tea.Cmd {
	return tea.Tick(2*time.Second, func(time.Time) tea.Msg {
		return statsTickMsg{id: id}
	})
}
