package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"dockertui/internal/component"

	btable "github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	mounttypes "github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/swarm"
)

type serviceAction int

const (
	serviceActionInspect serviceAction = iota
	serviceActionShowTasks
	serviceActionScale
	serviceActionRemove
	serviceActionUpdateImage
)

type serviceItem struct {
	ID       string
	Name     string
	Image    string
	Replicas string
}

type taskItem struct {
	ID           string
	Name         string
	Image        string
	CurrentState string
	Node         string
}

type servicesLoadedMsg struct {
	items []serviceItem
	err   error
}

type tasksLoadedMsg struct {
	items []taskItem
	err   error
}

type serviceUpdatedMsg struct {
	err              error
	inspectServiceID string
}

type serviceRemovedMsg struct {
	err error
}

type serviceInspectLoadedMsg struct {
	service swarm.Service
	title   string
	content string
	err     error
}

func newServiceActionTable() btable.Model {
	t := btable.New()
	setActionRows(&t, []btable.Row{
		{"[I] inspect"},
		{"[T] tasks"},
		{"[S] scale"},
		{"[R] remove"},
	})
	t.Focus()
	return t
}

func newServiceInput() textinput.Model {
	return component.NewTextInput(serviceInputModalWidth - 8)
}

func (m *model) setServiceActionTable() {
	setActionRows(&m.action, []btable.Row{
		{"[I] inspect"},
		{"[T] tasks"},
		{"[S] scale"},
		{"[R] remove"},
	})
}

func (m *model) setServiceTable() {
	rows := make([]btable.Row, 0, len(m.services))
	m.filteredServices = m.filteredServices[:0]
	for i, s := range m.services {
		if !matchesFilter(m.serviceFilter, s.ID, s.Name, s.Image, s.Replicas) {
			continue
		}
		m.filteredServices = append(m.filteredServices, i)
		rows = append(rows, btable.Row{s.ID, s.Name, s.Image, s.Replicas})
	}
	component.SetTableRows(&m.table, serviceColumns(m.rightWidth()), rows, filteredCursor(m.filteredServices, m.selectedService))
	m.resizeComponents()
	m.applyFocus()
}

func (m *model) setTaskTable() {
	rows := make([]btable.Row, 0, len(m.tasks))
	for _, t := range m.tasks {
		rows = append(rows, btable.Row{t.ID, t.Name, t.Image, t.CurrentState, t.Node})
	}
	component.SetTableRows(&m.table, taskColumns(m.rightWidth()), rows, clamp(m.selectedTask, len(rows)))
	m.resizeComponents()
	m.applyFocus()
}

func (m model) openServiceTasks() (tea.Model, tea.Cmd) {
	m.actionOpen = false
	m.stopLog()
	m.mode = viewTaskList
	m.tasks = nil
	m.selectedTask = 0
	m.setTaskTable()
	m.status = "Loading service tasks"
	return m, loadTasks(m.activeService.ID)
}

func (m model) openServiceAction() (tea.Model, tea.Cmd) {
	switch serviceAction(m.action.Cursor()) {
	case serviceActionScale:
		return m.openServiceInput(serviceActionScale)
	case serviceActionUpdateImage:
		return m.openServiceInput(serviceActionUpdateImage)
	case serviceActionInspect:
		return m.openServiceInspect()
	case serviceActionRemove:
		return m.openServiceRemoveConfirm()
	default:
		return m.openServiceTasks()
	}
}

func (m model) openServiceActionShortcut(key string) (tea.Model, tea.Cmd, bool) {
	switch key {
	case "i":
		m.action.SetCursor(int(serviceActionInspect))
	case "t":
		m.action.SetCursor(int(serviceActionShowTasks))
	case "s":
		m.action.SetCursor(int(serviceActionScale))
	case "r":
		m.action.SetCursor(int(serviceActionRemove))
	default:
		return m, nil, false
	}
	updated, cmd := m.openServiceAction()
	return updated, cmd, true
}

func (m model) openServiceRemoveConfirm() (tea.Model, tea.Cmd) {
	m.actionOpen = false
	m.serviceRemoveConfirmOpen = true
	m.status = "Confirm service remove"
	return m, nil
}

func (m model) openServiceInspect() (tea.Model, tea.Cmd) {
	m.actionOpen = false
	m.status = "Loading service inspect"
	return m, loadServiceInspect(m.activeService.ID)
}

func (m model) openServiceInput(action serviceAction) (tea.Model, tea.Cmd) {
	m.actionOpen = false
	m.inputOpen = true
	m.inputAction = action
	m.input.Reset()
	if action == serviceActionScale {
		m.input.Prompt = "Replicas: "
		m.input.Placeholder = "3"
		m.status = "Scale service"
	} else {
		m.input.Prompt = "Image: "
		m.input.Placeholder = m.activeService.Image
		m.status = "Update service image"
	}
	cmd := m.input.Focus()
	m.applyFocus()
	return m, cmd
}

func (m model) submitServiceInput() (tea.Model, tea.Cmd) {
	value := strings.TrimSpace(m.input.Value())
	if value == "" {
		return m, nil
	}
	m.inputOpen = false
	m.input.Blur()
	m.status = "Updating service"
	if m.inputAction == serviceActionScale {
		return m, scaleService(m.activeService.Name, value)
	}
	return m, updateServiceImage(m.activeService.Name, value)
}

func loadServices() tea.Cmd {
	return func() tea.Msg {
		out, err := dockerOutput("service", "ls", "--format", "{{.ID}}\t{{.Name}}\t{{.Image}}\t{{.Replicas}}")
		if err != nil {
			return servicesLoadedMsg{err: err}
		}
		var items []serviceItem
		for _, line := range splitLines(out) {
			cols := strings.Split(line, "\t")
			if len(cols) < 4 {
				continue
			}
			items = append(items, serviceItem{
				ID:       cols[0],
				Name:     cols[1],
				Image:    cols[2],
				Replicas: cols[3],
			})
		}
		return servicesLoadedMsg{items: items}
	}
}

func loadTasks(serviceID string) tea.Cmd {
	return func() tea.Msg {
		out, err := dockerOutput("service", "ps", serviceID, "--format", "{{.ID}}\t{{.Name}}\t{{.Image}}\t{{.CurrentState}}\t{{.Node}}")
		if err != nil {
			return tasksLoadedMsg{err: err}
		}
		var items []taskItem
		for _, line := range splitLines(out) {
			cols := strings.Split(line, "\t")
			if len(cols) < 5 {
				continue
			}
			items = append(items, taskItem{
				ID:           cols[0],
				Name:         cols[1],
				Image:        cols[2],
				CurrentState: cols[3],
				Node:         cols[4],
			})
		}
		return tasksLoadedMsg{items: items}
	}
}

func loadServiceInspect(serviceID string) tea.Cmd {
	return func() tea.Msg {
		cli, err := dockerAPIClient()
		if err != nil {
			return serviceInspectLoadedMsg{err: err}
		}
		defer cli.Close()

		ctx := context.Background()
		info, _, err := cli.ServiceInspectWithRaw(ctx, serviceID, swarm.ServiceInspectOptions{InsertDefaults: true})
		if err != nil {
			return serviceInspectLoadedMsg{err: err}
		}
		if services, err := cli.ServiceList(ctx, swarm.ServiceListOptions{Status: true}); err == nil {
			for _, s := range services {
				if s.ID == info.ID {
					info.ServiceStatus = s.ServiceStatus
					break
				}
			}
		}
		return serviceInspectLoadedMsg{
			service: info,
			title:   fmt.Sprintf("Service Inspect: %s", info.Spec.Name),
			content: formatServiceInspect(info),
		}
	}
}

func scaleService(serviceName, replicas string) tea.Cmd {
	return updateServiceReplicas(serviceName, replicas, "")
}

func updateServiceImage(serviceName, image string) tea.Cmd {
	return updateServiceImageSpec(serviceName, image, "")
}

func updateServiceInspectImage(serviceID, image string) tea.Cmd {
	return updateServiceImageSpec(serviceID, image, serviceID)
}

func updateServiceInspectReplicas(serviceID, replicas string) tea.Cmd {
	return updateServiceReplicas(serviceID, replicas, serviceID)
}

func updateServiceImageSpec(serviceID, image, inspectServiceID string) tea.Cmd {
	return func() tea.Msg {
		err := updateServiceSpec(serviceID, func(spec *swarm.ServiceSpec) error {
			ensureContainerSpec(spec).Image = image
			return nil
		})
		return serviceUpdatedMsg{err: err, inspectServiceID: inspectServiceID}
	}
}

func updateServiceReplicas(serviceID, replicas, inspectServiceID string) tea.Cmd {
	return func() tea.Msg {
		var count uint64
		if _, err := fmt.Sscanf(strings.TrimSpace(replicas), "%d", &count); err != nil {
			return serviceUpdatedMsg{err: err, inspectServiceID: inspectServiceID}
		}
		err := updateServiceSpec(serviceID, func(spec *swarm.ServiceSpec) error {
			spec.Mode = swarm.ServiceMode{Replicated: &swarm.ReplicatedService{Replicas: &count}}
			return nil
		})
		return serviceUpdatedMsg{err: err, inspectServiceID: inspectServiceID}
	}
}

func updateServiceInspectStaged(serviceID string, env []string, mounts []mounttypes.Mount, constraints []string, inspectServiceID string) tea.Cmd {
	return func() tea.Msg {
		err := updateServiceSpec(serviceID, func(spec *swarm.ServiceSpec) error {
			container := ensureContainerSpec(spec)
			container.Env = append([]string(nil), env...)
			container.Mounts = append([]mounttypes.Mount(nil), mounts...)
			placement := ensurePlacement(spec)
			placement.Constraints = append([]string(nil), constraints...)
			return nil
		})
		return serviceUpdatedMsg{err: err, inspectServiceID: inspectServiceID}
	}
}

func removeService(serviceID string) tea.Cmd {
	return func() tea.Msg {
		cli, err := dockerAPIClient()
		if err != nil {
			return serviceRemovedMsg{err: err}
		}
		defer cli.Close()

		err = cli.ServiceRemove(context.Background(), serviceID)
		return serviceRemovedMsg{err: err}
	}
}

func updateServiceSpec(serviceID string, mutate func(*swarm.ServiceSpec) error) error {
	cli, err := dockerAPIClient()
	if err != nil {
		return err
	}
	defer cli.Close()

	ctx := context.Background()
	info, _, err := cli.ServiceInspectWithRaw(ctx, serviceID, swarm.ServiceInspectOptions{InsertDefaults: true})
	if err != nil {
		return err
	}
	if err := mutate(&info.Spec); err != nil {
		return err
	}
	_, err = cli.ServiceUpdate(ctx, info.ID, info.Version, info.Spec, swarm.ServiceUpdateOptions{})
	return err
}

func ensureContainerSpec(spec *swarm.ServiceSpec) *swarm.ContainerSpec {
	if spec.TaskTemplate.ContainerSpec == nil {
		spec.TaskTemplate.ContainerSpec = &swarm.ContainerSpec{}
	}
	return spec.TaskTemplate.ContainerSpec
}

func ensurePlacement(spec *swarm.ServiceSpec) *swarm.Placement {
	if spec.TaskTemplate.Placement == nil {
		spec.TaskTemplate.Placement = &swarm.Placement{}
	}
	return spec.TaskTemplate.Placement
}

func formatServiceInspect(info swarm.Service) string {
	spec := info.Spec.TaskTemplate.ContainerSpec
	image := "-"
	env := []string(nil)
	var mounts []string
	if spec != nil {
		image = spec.Image
		env = spec.Env
		for _, mount := range spec.Mounts {
			mounts = append(mounts, formatServiceMount(mount))
		}
	}
	constraints := []string(nil)
	if info.Spec.TaskTemplate.Placement != nil {
		constraints = info.Spec.TaskTemplate.Placement.Constraints
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Name: %s\n", info.Spec.Name)
	fmt.Fprintf(&b, "Image: %s\n", image)
	fmt.Fprintf(&b, "Runtime: %s\n", formatDuration(time.Since(info.CreatedAt)))
	fmt.Fprintf(&b, "Replicas: %s\n\n", serviceReplicas(info))
	b.WriteString("Environment\n")
	b.WriteString(formatEnv(env))
	b.WriteString("\nMounts\n")
	b.WriteString(formatInspectList(mounts))
	b.WriteString("\nConstraints\n")
	b.WriteString(formatInspectList(constraints))
	return b.String()
}

func formatInspectList(items []string) string {
	if len(items) == 0 {
		return "  (none)\n"
	}
	var b strings.Builder
	for _, item := range items {
		fmt.Fprintf(&b, "  %s\n", item)
	}
	return b.String()
}

func serviceReplicas(info swarm.Service) string {
	if info.ServiceStatus != nil {
		return fmt.Sprintf("%d/%d", info.ServiceStatus.RunningTasks, info.ServiceStatus.DesiredTasks)
	}
	if info.Spec.Mode.Replicated != nil && info.Spec.Mode.Replicated.Replicas != nil {
		return fmt.Sprintf("%d", *info.Spec.Mode.Replicated.Replicas)
	}
	return "-"
}
