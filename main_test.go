package main

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	containertypes "github.com/docker/docker/api/types/container"
	mounttypes "github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/swarm"
)

func TestNavComponentShowsBothOptions(t *testing.T) {
	m := initialModel()
	m.width = 100
	m.height = 8
	m.resizeComponents()

	view := m.View()

	if !strings.Contains(view, "Containers") {
		t.Fatalf("view does not contain Containers option:\n%s", view)
	}
	if !strings.Contains(view, "Services") {
		t.Fatalf("view does not contain Services option:\n%s", view)
	}
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, "Containers") && strings.Contains(line, "Services") {
			return
		}
	}
	t.Fatalf("navigation options are not rendered horizontally:\n%s", view)
}

func TestAppTitleRendered(t *testing.T) {
	m := initialModel()
	m.width = 100
	m.height = 8
	m.resizeComponents()

	view := m.View()

	if !strings.Contains(view, "Docker TUI") {
		t.Fatalf("view does not contain app title:\n%s", view)
	}
}

func TestOperationHintRendered(t *testing.T) {
	m := initialModel()
	m.width = 100
	m.height = 8
	m.resizeComponents()

	view := m.View()

	if !strings.Contains(view, "Tab 切换导航") {
		t.Fatalf("view does not contain Tab hint:\n%s", view)
	}
	if !strings.Contains(view, "↑/↓ 切换列表选项") {
		t.Fatalf("view does not contain option navigation hint:\n%s", view)
	}
}

func TestMainPanelRendersBelowNavigation(t *testing.T) {
	m := initialModel()
	m.width = 100
	m.height = 8
	m.resizeComponents()

	navLine := -1
	mainLine := -1
	for i, line := range strings.Split(m.View(), "\n") {
		if strings.Contains(line, "╔═Navigation") {
			navLine = i
		}
		if strings.Contains(line, "┌─Containers") {
			mainLine = i
		}
	}
	if navLine < 0 || mainLine < 0 || mainLine <= navLine {
		t.Fatalf("main panel is not rendered below navigation:\n%s", m.View())
	}
}

func TestPanelsRenderTitlesInBorders(t *testing.T) {
	m := initialModel()
	m.width = 100
	m.height = 8
	m.resizeComponents()

	if !strings.Contains(m.View(), "╔═Navigation") {
		t.Fatalf("navigation title is not rendered in top border:\n%s", m.View())
	}
	if !strings.Contains(m.View(), "┌─Containers") {
		t.Fatalf("container title is not rendered in top border:\n%s", m.View())
	}
}

func TestActivePanelUsesDoubleBorder(t *testing.T) {
	m := initialModel()
	m.width = 100
	m.height = 8
	m.resizeComponents()

	if !strings.Contains(m.View(), "╔═Navigation") {
		t.Fatalf("active navigation panel does not use double border:\n%s", m.View())
	}

	updated, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyTab})
	next := updated.(model)

	if !strings.Contains(next.View(), "╔═Containers") {
		t.Fatalf("active main panel does not use double border:\n%s", next.View())
	}
}

func TestNavSelectionSwitchesToServices(t *testing.T) {
	m := initialModel()
	m.nav.Select(1)
	m.navSection = m.selectedNavSection()

	if m.navSection != sectionServices {
		t.Fatalf("nav section = %v, want services", m.navSection)
	}
}

func TestContainerLoadPopulatesTableComponent(t *testing.T) {
	m := initialModel()
	updated, _ := m.Update(containersLoadedMsg{
		items: []containerItem{{
			ID:      "abc123",
			Name:    "web",
			Image:   "nginx:latest",
			Command: `"nginx -g 'daemon off;'"`,
			Created: "2 hours ago",
			Status:  "Up 2 hours",
			Ports:   "0.0.0.0:8080->80/tcp",
		}},
	})

	next := updated.(model)
	if len(next.table.Rows()) != 1 {
		t.Fatalf("table row count = %d, want 1", len(next.table.Rows()))
	}
	row := next.table.Rows()[0]
	if len(row) != 7 {
		t.Fatalf("table column count = %d, want 7", len(row))
	}
	for i, expected := range []string{"abc123", "web", "nginx:latest", `"nginx -g 'daemon off;'"`, "2 hours ago", "Up 2 hours", "0.0.0.0:8080->80/tcp"} {
		if row[i] != expected {
			t.Fatalf("row[%d] = %q, want %q", i, row[i], expected)
		}
	}
}

func TestContainerFilterMapsToOriginalSelection(t *testing.T) {
	m := initialModel()
	m.mode = viewContainerList
	m.containers = []containerItem{
		{ID: "one", Name: "db", Image: "postgres", Status: "Up"},
		{ID: "two", Name: "api", Image: "nginx", Status: "Up"},
	}

	m.applyFilterValue("api")
	m.table.SetCursor(0)
	m.syncTableSelection()

	if len(m.table.Rows()) != 1 {
		t.Fatalf("filtered row count = %d, want 1", len(m.table.Rows()))
	}
	if m.selectedContainer != 1 {
		t.Fatalf("selected container index = %d, want 1", m.selectedContainer)
	}
}

func TestContainerSelectionOpensActionPopover(t *testing.T) {
	m := initialModel()
	m.mode = viewContainerList
	m.focus = focusMain
	m.containers = []containerItem{{
		ID:     "abc123",
		Name:   "web",
		Image:  "nginx:latest",
		Status: "Up",
	}}
	m.setContainerTable()

	updated, _ := m.openSelection()
	next := updated.(model)
	view := next.View()

	if next.mode != viewContainerList {
		t.Fatalf("mode = %v, want container list", next.mode)
	}
	if next.activeContainer.Name != "web" {
		t.Fatalf("active container = %q, want web", next.activeContainer.Name)
	}
	if !next.actionOpen {
		t.Fatalf("container action popover is not open")
	}
	for _, option := range []string{"[R] restart", "[S] stop", "[K] kill", "[T] start", "[L] logs", "[I] inspect"} {
		if !strings.Contains(view, option) {
			t.Fatalf("container action popover missing %s option:\n%s", option, view)
		}
	}
}

func TestContainerActionShortcutUsesNextLetterForConflict(t *testing.T) {
	m := initialModel()
	m.mode = viewContainerList
	m.activeContainer = containerItem{ID: "abc123", Name: "web"}
	m.actionOpen = true
	m.setContainerActionTable()

	updated, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	next := updated.(model)

	if next.actionOpen {
		t.Fatalf("container action popover is still open")
	}
	if next.status != "Running docker start" {
		t.Fatalf("status = %q, want Running docker start", next.status)
	}
	if cmd == nil {
		t.Fatalf("container start command is nil")
	}
}

func TestContainerActionReturnsUpdateCommand(t *testing.T) {
	m := initialModel()
	m.mode = viewContainerList
	m.activeContainer = containerItem{ID: "abc123", Name: "web"}
	m.actionOpen = true

	updated, cmd := m.runContainerAction("restart")
	next := updated.(model)

	if next.actionOpen {
		t.Fatalf("container action popover is still open")
	}
	if next.status != "Running docker restart" {
		t.Fatalf("status = %q, want Running docker restart", next.status)
	}
	if cmd == nil {
		t.Fatalf("container update command is nil")
	}
}

func TestContainerLogModalRendersOverContainerList(t *testing.T) {
	m := initialModel()
	m.width = 120
	m.height = 30
	m.mode = viewContainerList
	m.activeContainer = containerItem{Name: "web"}
	m.logs = []string{"container log line"}
	m.taskLogOpen = true
	m.logTitle = "Container Logs: web  Esc close"
	m.setContainerTable()
	m.resizeComponents()
	m.updateLogView()

	view := m.View()

	if !strings.Contains(view, "Container Logs: web") {
		t.Fatalf("container log modal title missing:\n%s", view)
	}
	if !strings.Contains(view, "container log line") {
		t.Fatalf("container log content missing:\n%s", view)
	}

	modal := m.taskLogModalView()
	modalLines := strings.Split(modal, "\n")
	if strings.TrimSpace(modalLines[0]) != "" {
		t.Fatalf("log modal missing blank top margin:\n%s", modal)
	}
	if !strings.Contains(modal, "┌─Container Logs: web") {
		t.Fatalf("container log modal title is not rendered in top border:\n%s", modal)
	}
}

func TestClosingContainerLogModalRestoresListSelection(t *testing.T) {
	m := initialModel()
	m.width = 120
	m.height = 30
	m.mode = viewContainerList
	m.focus = focusMain
	m.containers = []containerItem{
		{ID: "one", Name: "web", Image: "nginx", Status: "Up"},
		{ID: "two", Name: "api", Image: "nginx", Status: "Up"},
	}
	m.setContainerTable()
	m.table.Blur()
	m.taskLogOpen = true
	m.logStatus = "2 containers"

	updated, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyEsc})
	next := updated.(model)
	if !next.table.Focused() {
		t.Fatalf("container list table is not focused after closing log modal")
	}

	updated, _ = next.handleKey(tea.KeyMsg{Type: tea.KeyDown})
	next = updated.(model)
	if next.table.Cursor() != 1 {
		t.Fatalf("table cursor = %d, want 1", next.table.Cursor())
	}
}

func TestFormatContainerInspectIncludesChartAndEnv(t *testing.T) {
	content := formatContainerInspect(containertypes.InspectResponse{
		ContainerJSONBase: &containertypes.ContainerJSONBase{
			Name: "/web",
			State: &containertypes.State{
				Running:   true,
				StartedAt: time.Now().Add(-2 * time.Hour).Format(time.RFC3339Nano),
			},
		},
		Config: &containertypes.Config{
			Image: "nginx:latest",
			Env:   []string{"APP_ENV=test"},
		},
	}, containertypes.StatsResponse{
		CPUStats: containertypes.CPUStats{
			CPUUsage:    containertypes.CPUUsage{TotalUsage: 200, PercpuUsage: []uint64{1, 1}},
			SystemUsage: 2000,
			OnlineCPUs:  2,
		},
		PreCPUStats: containertypes.CPUStats{
			CPUUsage:    containertypes.CPUUsage{TotalUsage: 100},
			SystemUsage: 1000,
		},
		MemoryStats: containertypes.MemoryStats{
			Usage: 512,
			Limit: 1024,
		},
	}, true)

	for _, expected := range []string{"Name: web", "Image: nginx:latest", "Runtime:", "APP_ENV=test", "Resource Chart", "CPU", "Memory"} {
		if !strings.Contains(content, expected) {
			t.Fatalf("container inspect content missing %q:\n%s", expected, content)
		}
	}
}

func TestFormatServiceInspectIncludesReplicasAndEnv(t *testing.T) {
	replicas := uint64(3)
	content := formatServiceInspect(swarm.Service{
		Meta: swarm.Meta{CreatedAt: time.Now().Add(-time.Hour)},
		Spec: swarm.ServiceSpec{
			Annotations: swarm.Annotations{Name: "api"},
			TaskTemplate: swarm.TaskSpec{
				ContainerSpec: &swarm.ContainerSpec{
					Image: "example/api:latest",
					Env:   []string{"APP_ENV=prod"},
					Mounts: []mounttypes.Mount{{
						Type:     mounttypes.TypeBind,
						Source:   "/host/data",
						Target:   "/data",
						ReadOnly: true,
					}},
				},
				Placement: &swarm.Placement{Constraints: []string{"node.labels.disk == ssd"}},
			},
			Mode: swarm.ServiceMode{
				Replicated: &swarm.ReplicatedService{Replicas: &replicas},
			},
		},
		ServiceStatus: &swarm.ServiceStatus{RunningTasks: 2, DesiredTasks: 3},
	})

	for _, expected := range []string{"Name: api", "Image: example/api:latest", "Runtime:", "Replicas: 2/3", "APP_ENV=prod", "Mounts", "bind /host/data -> /data (ro)", "Constraints", "node.labels.disk == ssd"} {
		if !strings.Contains(content, expected) {
			t.Fatalf("service inspect content missing %q:\n%s", expected, content)
		}
	}
}

func TestServiceInspectCursorMovesBetweenEditableRows(t *testing.T) {
	m := modelWithServiceInspect()

	if row := m.serviceInspect.selectedRow(); row.kind != serviceInspectRowImage {
		t.Fatalf("selected row = %v, want image", row.kind)
	}

	updated, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyDown})
	next := updated.(model)
	if row := next.serviceInspect.selectedRow(); row.kind != serviceInspectRowReplicas {
		t.Fatalf("selected row = %v, want replicas", row.kind)
	}
}

func TestServiceInspectEnvUpdateIsStaged(t *testing.T) {
	m := modelWithServiceInspect()
	m.serviceInspect.cursor = serviceInspectRowIndex(m.serviceInspect, serviceInspectRowEnv)
	m.refreshServiceInspect()

	updated, _ := m.openServiceInspectSelection()
	next := updated.(model)
	if !next.actionOpen {
		t.Fatalf("inspect action popover is not open")
	}

	updated, _ = next.openServiceInspectAction()
	next = updated.(model)
	if !next.inputOpen {
		t.Fatalf("inspect input is not open")
	}

	next.input.SetValue("APP_ENV=stage")
	updated, cmd := next.submitServiceInspectInput()
	next = updated.(model)
	if cmd != nil {
		t.Fatalf("staged env update returned command")
	}
	if !next.serviceInspect.dirty {
		t.Fatalf("service inspect change was not staged")
	}
	if next.serviceInspect.env[0] != "APP_ENV=stage" {
		t.Fatalf("env = %q, want APP_ENV=stage", next.serviceInspect.env[0])
	}
}

func TestServiceInspectSaveChangeBuildsCommand(t *testing.T) {
	m := modelWithServiceInspect()
	m.serviceInspect.addEnv("DEBUG=true")
	m.serviceInspect.cursor = serviceInspectRowIndex(m.serviceInspect, serviceInspectRowSave)
	m.refreshServiceInspect()

	updated, _ := m.openServiceInspectSelection()
	next := updated.(model)
	if !next.confirmOpen {
		t.Fatalf("save confirmation is not open")
	}

	updated, cmd := next.confirmServiceInspectChanges()
	next = updated.(model)
	if cmd == nil {
		t.Fatalf("save confirmation did not return update command")
	}
	if !next.serviceInspect.dirty {
		t.Fatalf("service inspect changes were cleared before update completed")
	}
}

func TestParseMountSpec(t *testing.T) {
	mount, err := parseMountSpec("type=bind,source=/host,target=/data,readonly=true")
	if err != nil {
		t.Fatal(err)
	}
	if mount.Type != mounttypes.TypeBind || mount.Source != "/host" || mount.Target != "/data" || !mount.ReadOnly {
		t.Fatalf("mount = %#v", mount)
	}
}

func TestInspectPanelHasPadding(t *testing.T) {
	m := modelWithServiceInspect()
	view := m.mainComponentView()
	if !strings.HasPrefix(view, " ") {
		t.Fatalf("inspect panel view does not start with padding:\n%s", view)
	}
}

func modelWithServiceInspect() model {
	replicas := uint64(2)
	service := swarm.Service{
		Meta: swarm.Meta{CreatedAt: time.Now().Add(-time.Hour)},
		ID:   "svc123",
		Spec: swarm.ServiceSpec{
			Annotations: swarm.Annotations{Name: "api"},
			TaskTemplate: swarm.TaskSpec{
				ContainerSpec: &swarm.ContainerSpec{
					Image: "example/api:latest",
					Env:   []string{"APP_ENV=prod"},
					Mounts: []mounttypes.Mount{{
						Type:   mounttypes.TypeBind,
						Source: "/host/data",
						Target: "/data",
					}},
				},
				Placement: &swarm.Placement{Constraints: []string{"node.labels.disk == ssd"}},
			},
			Mode: swarm.ServiceMode{Replicated: &swarm.ReplicatedService{Replicas: &replicas}},
		},
	}
	m := initialModel()
	m.width = 120
	m.height = 30
	m.mode = viewInspect
	m.focus = focusMain
	m.inspectFrom = sectionServices
	m.inspectTitle = "Service Inspect: api"
	m.serviceInspect = newServiceInspectState(service)
	m.resizeComponents()
	m.refreshServiceInspect()
	return m
}

func serviceInspectRowIndex(state serviceInspectState, kind serviceInspectRowKind) int {
	for i, row := range state.rows {
		if row.kind == kind {
			return i
		}
	}
	return 0
}

func TestServiceFilterMapsToOriginalSelection(t *testing.T) {
	m := initialModel()
	m.mode = viewServiceList
	m.services = []serviceItem{
		{ID: "one", Name: "worker", Image: "example/worker", Replicas: "1/1"},
		{ID: "two", Name: "api", Image: "example/api", Replicas: "2/2"},
	}

	m.applyFilterValue("api")
	m.table.SetCursor(0)
	m.syncTableSelection()

	if len(m.table.Rows()) != 1 {
		t.Fatalf("filtered row count = %d, want 1", len(m.table.Rows()))
	}
	if m.selectedService != 1 {
		t.Fatalf("selected service index = %d, want 1", m.selectedService)
	}
}

func TestServiceSelectionOpensActionPopover(t *testing.T) {
	m := initialModel()
	m.mode = viewServiceList
	m.focus = focusMain
	m.services = []serviceItem{{
		ID:       "svc123",
		Name:     "api",
		Image:    "example/api:latest",
		Replicas: "1/1",
	}}
	m.setServiceTable()

	updated, _ := m.openSelection()
	next := updated.(model)

	if next.mode != viewServiceList {
		t.Fatalf("mode = %v, want service list", next.mode)
	}
	if next.activeService.Name != "api" {
		t.Fatalf("active service = %q, want api", next.activeService.Name)
	}
	if !next.actionOpen {
		t.Fatalf("service action popover is not open")
	}
	view := next.View()
	if !strings.Contains(view, "[T] tasks") {
		t.Fatalf("service action popover missing tasks option:\n%s", view)
	}
	if !strings.Contains(view, "[S] scale") {
		t.Fatalf("service action popover missing scale option:\n%s", view)
	}
	if strings.Contains(view, "update image") {
		t.Fatalf("service action popover still shows update image option:\n%s", view)
	}
	if !strings.Contains(view, "[I] inspect") {
		t.Fatalf("service action popover missing inspect option:\n%s", view)
	}
	if !strings.Contains(view, "[R] remove") {
		t.Fatalf("service action popover missing remove option:\n%s", view)
	}
}

func TestServiceRemoveActionOpensConfirm(t *testing.T) {
	m := initialModel()
	m.mode = viewServiceList
	m.focus = focusMain
	m.services = []serviceItem{{
		ID:       "svc123",
		Name:     "api",
		Image:    "example/api:latest",
		Replicas: "1/1",
	}}
	m.setServiceTable()

	updated, _ := m.openSelection()
	next := updated.(model)
	next.action.SetCursor(int(serviceActionRemove))
	updated, _ = next.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	next = updated.(model)

	if !next.serviceRemoveConfirmOpen {
		t.Fatalf("service remove confirmation is not open")
	}
	if !strings.Contains(next.View(), "Remove service?") {
		t.Fatalf("service remove confirmation title missing:\n%s", next.View())
	}
	if !strings.Contains(next.View(), "api") {
		t.Fatalf("service remove confirmation service name missing:\n%s", next.View())
	}
}

func TestServiceRemoveConfirmReturnsCommand(t *testing.T) {
	m := initialModel()
	m.activeService = serviceItem{ID: "svc123", Name: "api"}
	m.serviceRemoveConfirmOpen = true

	updated, cmd := m.confirmServiceRemove()
	next := updated.(model)

	if next.serviceRemoveConfirmOpen {
		t.Fatalf("service remove confirmation is still open")
	}
	if next.status != "Removing service" {
		t.Fatalf("status = %q, want Removing service", next.status)
	}
	if cmd == nil {
		t.Fatalf("service remove command is nil")
	}
}

func TestServiceRemovedMsgRefreshesServices(t *testing.T) {
	m := initialModel()

	updated, cmd := m.Update(serviceRemovedMsg{})
	next := updated.(model)

	if next.status != "Service removed" {
		t.Fatalf("status = %q, want Service removed", next.status)
	}
	if cmd == nil {
		t.Fatalf("service removed did not return refresh command")
	}
}

func TestServiceActionOpensTaskList(t *testing.T) {
	m := initialModel()
	m.mode = viewServiceList
	m.focus = focusMain
	m.services = []serviceItem{{
		ID:       "svc123",
		Name:     "api",
		Image:    "example/api:latest",
		Replicas: "1/1",
	}}
	m.setServiceTable()

	updated, _ := m.openSelection()
	updated, _ = updated.(model).handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	next := updated.(model)

	if next.mode != viewTaskList {
		t.Fatalf("mode = %v, want task list", next.mode)
	}
	if next.actionOpen {
		t.Fatalf("service action popover is still open")
	}
	if next.status != "Loading service tasks" {
		t.Fatalf("status = %q, want Loading service tasks", next.status)
	}
}

func TestServiceScaleActionOpensInputModal(t *testing.T) {
	m := initialModel()
	m.width = 120
	m.height = 30
	m.mode = viewServiceList
	m.focus = focusMain
	m.services = []serviceItem{{
		ID:       "svc123",
		Name:     "api",
		Image:    "example/api:latest",
		Replicas: "1/1",
	}}
	m.setServiceTable()

	updated, _ := m.openSelection()
	next := updated.(model)
	updated, _ = next.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	next = updated.(model)

	if !next.inputOpen {
		t.Fatalf("service input modal is not open")
	}
	if next.inputAction != serviceActionScale {
		t.Fatalf("input action = %v, want scale", next.inputAction)
	}
	if !strings.Contains(next.View(), "Scale service: api") {
		t.Fatalf("scale input modal title missing:\n%s", next.View())
	}
}

func TestSpaceRefreshesCurrentList(t *testing.T) {
	m := initialModel()
	m.mode = viewServiceList
	m.focus = focusMain

	updated, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeySpace})
	next := updated.(model)

	if next.status != "Loading services" {
		t.Fatalf("status = %q, want Loading services", next.status)
	}
	if cmd == nil {
		t.Fatalf("space refresh command is nil")
	}
}

func TestServiceInputSubmitReturnsUpdateCommand(t *testing.T) {
	m := initialModel()
	m.activeService = serviceItem{Name: "api"}
	m.inputOpen = true
	m.inputAction = serviceActionScale
	m.input.SetValue("3")

	updated, cmd := m.submitServiceInput()
	next := updated.(model)

	if next.inputOpen {
		t.Fatalf("service input modal is still open")
	}
	if next.status != "Updating service" {
		t.Fatalf("status = %q, want Updating service", next.status)
	}
	if cmd == nil {
		t.Fatalf("update command is nil")
	}
}

func TestTaskListShowsServiceTitle(t *testing.T) {
	m := initialModel()
	m.width = 120
	m.height = 30
	m.mode = viewTaskList
	m.activeService = serviceItem{Name: "api"}
	m.setTaskTable()

	view := m.View()

	if !strings.Contains(view, "Tasks for service: api") {
		t.Fatalf("task list title missing:\n%s", view)
	}
}

func TestLogBufferKeepsLastHundredLines(t *testing.T) {
	m := initialModel()
	m.mode = viewTaskList
	m.taskLogOpen = true
	m.logSession = &logSession{
		id:     1,
		lines:  make(chan string),
		cancel: func() {},
	}

	var updated tea.Model = m
	for i := 0; i < 101; i++ {
		updated, _ = updated.Update(logLineMsg{id: 1, line: fmt.Sprintf("line-%03d", i)})
	}

	next := updated.(model)
	if len(next.logs) != 100 {
		t.Fatalf("log count = %d, want 100", len(next.logs))
	}
	if next.logs[0] != "line-001" {
		t.Fatalf("first log = %q, want line-001", next.logs[0])
	}
}

func TestTaskLogModalHasFixedSize(t *testing.T) {
	m := initialModel()
	m.width = 160
	m.height = 40

	w, h := m.taskLogModalSize()

	if w != taskLogModalWidth {
		t.Fatalf("modal width = %d, want %d", w, taskLogModalWidth)
	}
	if h != taskLogModalHeight {
		t.Fatalf("modal height = %d, want %d", h, taskLogModalHeight)
	}
}

func TestTaskLogPrefixIsRemoved(t *testing.T) {
	m := initialModel()
	m.mode = viewTaskList
	m.taskLogOpen = true
	m.logSession = &logSession{
		id:          1,
		lines:       make(chan string),
		cancel:      func() {},
		stripPrefix: true,
	}

	updated, _ := m.Update(logLineMsg{
		id:   1,
		line: "flypower_rufus_account_test.18.s1jz4ohmmoff@iZ0xiav0vkua84ajh65ue0Z | log body",
	})

	next := updated.(model)
	if next.logs[0] != "log body" {
		t.Fatalf("log line = %q, want log body", next.logs[0])
	}
}

func TestTaskLogModalRendersOverTaskList(t *testing.T) {
	m := initialModel()
	m.width = 120
	m.height = 30
	m.mode = viewTaskList
	m.activeService = serviceItem{Name: "api"}
	m.activeTask = taskItem{Name: "api.1"}
	m.tasks = []taskItem{m.activeTask}
	m.logs = []string{"task log line"}
	m.taskLogOpen = true
	m.setTaskTable()
	m.resizeComponents()
	m.updateLogView()

	view := m.View()

	if m.mode != viewTaskList {
		t.Fatalf("mode = %v, want task list behind modal", m.mode)
	}
	if !strings.Contains(view, "Task Logs: api.1") {
		t.Fatalf("modal title missing:\n%s", view)
	}
	if !strings.Contains(view, "task log line") {
		t.Fatalf("modal log content missing:\n%s", view)
	}
}

func TestEscClosesTaskLogModal(t *testing.T) {
	m := initialModel()
	m.mode = viewTaskList
	m.activeService = serviceItem{Name: "api"}
	m.taskLogOpen = true
	m.logSession = &logSession{
		id:     1,
		lines:  make(chan string),
		cancel: func() {},
	}

	updated, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyEsc})
	next := updated.(model)

	if next.taskLogOpen {
		t.Fatalf("task log modal is still open")
	}
	if next.logSession != nil {
		t.Fatalf("log session was not stopped")
	}
}

func TestEscFromTaskListReturnsServiceListAfterTaskLogModal(t *testing.T) {
	m := initialModel()
	m.width = 120
	m.height = 30
	m.mode = viewTaskList
	m.focus = focusMain
	m.services = []serviceItem{{
		ID:       "svc123",
		Name:     "api",
		Image:    "example/api:latest",
		Replicas: "1/1",
	}}
	m.activeService = m.services[0]
	m.tasks = []taskItem{{
		ID:           "task123",
		Name:         "api.1",
		Image:        "example/api:latest",
		CurrentState: "Running",
		Node:         "node-1",
	}}
	m.setTaskTable()
	m.taskLogOpen = true

	updated, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyEsc})
	updated, _ = updated.(model).handleKey(tea.KeyMsg{Type: tea.KeyEsc})
	next := updated.(model)

	if next.mode != viewServiceList {
		t.Fatalf("mode = %v, want service list", next.mode)
	}
	if next.taskLogOpen {
		t.Fatalf("task log modal is still open")
	}
	_ = next.View()
}
