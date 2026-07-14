package main

import (
	"dockertui/internal/component"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	btable "github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

type section int

const (
	sectionContainers section = iota
	sectionServices
)

type focus int

const (
	focusNav focus = iota
	focusMain
)

type viewMode int

const (
	viewContainerList viewMode = iota
	viewContainerDetail
	viewServiceList
	viewTaskList
	viewTaskLog
	viewInspect
)

const (
	taskLogModalWidth  = 120
	taskLogModalHeight = 32
)

const (
	appTitleHeight            = 1
	operationHintHeight       = 1
	navPanelHeight            = 3
	listPanelPadding          = 1
	serviceActionPopoverWidth = 22
	serviceInputModalWidth    = 64
)

type navItem struct {
	section section
	title   string
}

func (i navItem) Title() string       { return i.title }
func (i navItem) Description() string { return "" }
func (i navItem) FilterValue() string { return i.title }

type model struct {
	width  int
	height int

	nav       list.Model
	table     btable.Model
	infoTable btable.Model
	action    btable.Model
	filter    textinput.Model
	input     textinput.Model
	logView   viewport.Model
	inspect   viewport.Model

	navSection section
	focus      focus
	mode       viewMode

	containers         []containerItem
	filteredContainers []int
	selectedContainer  int
	activeContainer    containerItem
	cpu                string
	memory             string
	statsID            int

	services         []serviceItem
	filteredServices []int
	selectedService  int
	activeService    serviceItem
	containerFilter  string
	serviceFilter    string

	tasks        []taskItem
	selectedTask int
	activeTask   taskItem

	logs                     []string
	logID                    int
	logSession               *logSession
	taskLogOpen              bool
	logTitle                 string
	logStatus                string
	inspectFrom              section
	inspectTitle             string
	serviceInspect           serviceInspectState
	inspectInput             serviceInspectInputState
	confirmOpen              bool
	confirmExit              bool
	serviceRemoveConfirmOpen bool
	actionOpen               bool
	inputOpen                bool
	inputAction              serviceAction

	status string
}

func initialModel() model {
	m := model{
		nav:        newNav(),
		table:      btable.New(),
		infoTable:  btable.New(),
		action:     newServiceActionTable(),
		filter:     component.NewFilterInput(),
		input:      newServiceInput(),
		logView:    viewport.New(1, 1),
		inspect:    viewport.New(1, 1),
		navSection: sectionContainers,
		focus:      focusNav,
		mode:       viewContainerList,
		status:     "Loading containers",
	}
	m.setContainerTable()
	m.updateInfoTable()
	return m
}

func newNav() list.Model {
	delegate := component.NewNavDelegate(22)
	items := []list.Item{
		navItem{section: sectionContainers, title: "Containers"},
		navItem{section: sectionServices, title: "Services"},
	}
	nav := list.New(items, delegate, 24, 10)
	nav.SetShowTitle(false)
	nav.SetShowStatusBar(false)
	nav.SetFilteringEnabled(false)
	nav.SetShowPagination(false)
	nav.SetShowHelp(false)
	return nav
}

func setActionRows(t *btable.Model, rows []btable.Row) {
	component.SetActionRows(t, rows, serviceActionPopoverWidth)
}

func (m model) Init() tea.Cmd {
	return loadContainers()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resizeComponents()
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	case containersLoadedMsg:
		if msg.err != nil {
			m.status = msg.err.Error()
			m.containers = nil
			m.setContainerTable()
			return m, nil
		}
		m.containers = msg.items
		m.selectedContainer = clamp(m.selectedContainer, len(m.containers))
		m.status = fmt.Sprintf("%d containers", len(m.containers))
		if m.mode == viewContainerList {
			m.setContainerTable()
		}
		return m, nil
	case servicesLoadedMsg:
		if msg.err != nil {
			m.status = msg.err.Error()
			m.services = nil
			m.setServiceTable()
			return m, nil
		}
		m.services = msg.items
		m.selectedService = clamp(m.selectedService, len(m.services))
		m.status = fmt.Sprintf("%d services", len(m.services))
		if m.mode == viewServiceList {
			m.setServiceTable()
		}
		return m, nil
	case tasksLoadedMsg:
		if msg.err != nil {
			m.status = msg.err.Error()
			m.tasks = nil
			m.setTaskTable()
			return m, nil
		}
		m.tasks = msg.items
		m.selectedTask = clamp(m.selectedTask, len(m.tasks))
		m.status = fmt.Sprintf("%d tasks for %s", len(m.tasks), m.activeService.Name)
		if m.mode == viewTaskList {
			m.setTaskTable()
		}
		return m, nil
	case serviceUpdatedMsg:
		if msg.err != nil {
			m.status = msg.err.Error()
			return m, nil
		}
		m.status = "Service updated"
		if msg.inspectServiceID != "" {
			return m, loadServiceInspect(msg.inspectServiceID)
		}
		return m, loadServices()
	case serviceRemovedMsg:
		if msg.err != nil {
			m.status = msg.err.Error()
			return m, nil
		}
		m.status = "Service removed"
		return m, loadServices()
	case containerUpdatedMsg:
		if msg.err != nil {
			m.status = msg.err.Error()
			return m, nil
		}
		m.status = "Container updated"
		return m, loadContainers()
	case inspectLoadedMsg:
		if msg.err != nil {
			m.status = msg.err.Error()
			return m, nil
		}
		m.mode = viewInspect
		m.focus = focusMain
		m.inspectFrom = msg.from
		m.inspectTitle = msg.title
		m.inspect.SetContent(msg.content)
		m.inspect.GotoTop()
		m.serviceInspect = serviceInspectState{}
		m.status = "Inspect loaded"
		m.resizeComponents()
		m.applyFocus()
		return m, nil
	case serviceInspectLoadedMsg:
		if msg.err != nil {
			m.status = msg.err.Error()
			return m, nil
		}
		m.mode = viewInspect
		m.focus = focusMain
		m.inspectFrom = sectionServices
		m.inspectTitle = msg.title
		m.serviceInspect = newServiceInspectState(msg.service)
		m.refreshServiceInspect()
		m.inspect.GotoTop()
		m.status = "Inspect loaded"
		m.resizeComponents()
		m.applyFocus()
		return m, nil
	case statsTickMsg:
		if m.mode != viewContainerDetail || msg.id != m.statsID {
			return m, nil
		}
		return m, tea.Batch(loadStats(msg.id, m.activeContainer.ID), tickStats(msg.id))
	case statsLoadedMsg:
		if m.mode != viewContainerDetail || msg.id != m.statsID {
			return m, nil
		}
		if msg.err != nil {
			m.cpu = "-"
			m.memory = "-"
			m.status = msg.err.Error()
		} else {
			m.cpu = msg.cpu
			m.memory = msg.memory
		}
		m.updateInfoTable()
		return m, nil
	case logLineMsg:
		if m.logSession == nil || msg.id != m.logSession.id {
			return m, nil
		}
		line := msg.line
		if m.logSession.stripPrefix {
			line = stripServiceLogPrefix(line)
		}
		m.logs = append(m.logs, line)
		if len(m.logs) > 100 {
			m.logs = m.logs[len(m.logs)-100:]
		}
		m.updateLogView()
		return m, waitLogLine(msg.id, m.logSession.lines)
	case logClosedMsg:
		if m.logSession != nil && msg.id == m.logSession.id {
			m.status = "Log stream closed"
			m.logSession = nil
		}
		return m, nil
	}
	return m, nil
}

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		m.stopLog()
		return m, tea.Quit
	case "esc":
		if m.serviceRemoveConfirmOpen {
			m.serviceRemoveConfirmOpen = false
			m.applyFocus()
			return m, nil
		}
		if m.confirmOpen {
			m.confirmOpen = false
			return m, nil
		}
		if m.taskLogOpen {
			return m.closeTaskLogModal()
		}
		if m.actionOpen {
			m.actionOpen = false
			m.applyFocus()
			return m, nil
		}
		if m.inputOpen {
			m.inputOpen = false
			m.inspectInput = serviceInspectInputState{}
			m.input.Blur()
			m.applyFocus()
			return m, nil
		}
		if m.filter.Focused() {
			m.filter.Blur()
			m.applyFocus()
			return m, nil
		}
		return m.goBack()
	}

	if m.taskLogOpen {
		var cmd tea.Cmd
		m.logView, cmd = m.logView.Update(msg)
		return m, cmd
	}

	if m.serviceRemoveConfirmOpen {
		if msg.String() == "enter" {
			return m.confirmServiceRemove()
		}
		return m, nil
	}

	if m.confirmOpen {
		if msg.String() == "enter" {
			return m.confirmServiceInspectChanges()
		}
		return m, nil
	}

	if m.actionOpen {
		switch msg.String() {
		case "enter":
			if m.mode == viewInspect && m.serviceInspect.active {
				return m.openServiceInspectAction()
			}
			if m.mode == viewContainerList {
				return m.openContainerAction()
			}
			return m.openServiceAction()
		case "q":
			m.stopLog()
			return m, tea.Quit
		}
		key := strings.ToLower(msg.String())
		if m.mode == viewContainerList {
			if updated, cmd, ok := m.openContainerActionShortcut(key); ok {
				return updated, cmd
			}
		}
		if m.mode == viewServiceList {
			if updated, cmd, ok := m.openServiceActionShortcut(key); ok {
				return updated, cmd
			}
		}
		var cmd tea.Cmd
		m.action, cmd = m.action.Update(msg)
		return m, cmd
	}

	if m.inputOpen {
		if msg.String() == "enter" {
			if m.inspectInput.active {
				return m.submitServiceInspectInput()
			}
			return m.submitServiceInput()
		}
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}

	if m.filter.Focused() {
		if msg.String() == "enter" {
			m.filter.Blur()
			m.applyFocus()
			return m, nil
		}
		var cmd tea.Cmd
		m.filter, cmd = m.filter.Update(msg)
		m.applyFilterValue(m.filter.Value())
		return m, cmd
	}

	switch msg.String() {
	case "q":
		m.stopLog()
		return m, tea.Quit
	case "tab":
		if m.focus == focusNav {
			m.focus = focusMain
		} else {
			m.focus = focusNav
		}
		m.applyFocus()
		return m, nil
	case "left":
		if m.focus == focusNav {
			return m.moveNav(-1)
		}
		m.focus = focusNav
		m.applyFocus()
		return m, nil
	case "right":
		if m.focus == focusNav {
			return m.moveNav(1)
		}
		m.focus = focusMain
		m.applyFocus()
		return m, nil
	case "r", " ":
		return m.refresh()
	case "enter":
		if m.mode == viewInspect && m.serviceInspect.active {
			return m.openServiceInspectSelection()
		}
		return m.openSelection()
	case "/":
		return m.focusFilter()
	}
	return m.updateFocusedComponent(msg)
}

func (m model) moveNav(delta int) (tea.Model, tea.Cmd) {
	old := m.navSection
	m.nav.Select(clamp(m.nav.Index()+delta, len(m.nav.Items())))
	m.navSection = m.selectedNavSection()
	if old != m.navSection {
		var cmd tea.Cmd
		m, cmd = m.showNavSection()
		return m, cmd
	}
	return m, nil
}

func (m model) updateFocusedComponent(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.focus == focusNav {
		old := m.navSection
		var cmd tea.Cmd
		m.nav, cmd = m.nav.Update(msg)
		m.navSection = m.selectedNavSection()
		if old != m.navSection {
			var load tea.Cmd
			m, load = m.showNavSection()
			return m, tea.Batch(cmd, load)
		}
		return m, cmd
	}

	switch m.mode {
	case viewContainerList, viewServiceList, viewTaskList:
		var cmd tea.Cmd
		m.table, cmd = m.table.Update(msg)
		m.syncTableSelection()
		return m, cmd
	case viewContainerDetail, viewTaskLog:
		var cmd tea.Cmd
		m.logView, cmd = m.logView.Update(msg)
		return m, cmd
	case viewInspect:
		if m.serviceInspect.active {
			return m.updateServiceInspectCursor(msg)
		}
		var cmd tea.Cmd
		m.inspect, cmd = m.inspect.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m model) selectedNavSection() section {
	item, ok := m.nav.SelectedItem().(navItem)
	if !ok {
		return sectionContainers
	}
	return item.section
}

func (m model) focusFilter() (tea.Model, tea.Cmd) {
	if m.mode != viewContainerList && m.mode != viewServiceList {
		return m, nil
	}
	m.focus = focusMain
	m.filter.SetValue(m.currentFilterValue())
	cmd := m.filter.Focus()
	m.applyFocus()
	return m, cmd
}

func (m model) currentFilterValue() string {
	if m.mode == viewServiceList {
		return m.serviceFilter
	}
	return m.containerFilter
}

func (m *model) applyFilterValue(value string) {
	switch m.mode {
	case viewContainerList:
		m.containerFilter = value
		m.setContainerTable()
	case viewServiceList:
		m.serviceFilter = value
		m.setServiceTable()
	}
}

func (m *model) syncFilterInput() {
	m.filter.SetValue(m.currentFilterValue())
}

func (m model) openSelection() (tea.Model, tea.Cmd) {
	if m.focus == focusNav {
		var cmd tea.Cmd
		m, cmd = m.showNavSection()
		m.focus = focusMain
		m.applyFocus()
		return m, cmd
	}

	switch m.mode {
	case viewContainerList:
		if len(m.filteredContainers) == 0 {
			return m, nil
		}
		m.syncTableSelection()
		m.activeContainer = m.containers[m.selectedContainer]
		m.setContainerActionTable()
		m.actionOpen = true
		m.action.SetCursor(0)
		m.action.Focus()
		m.table.Blur()
		m.status = "Select container action"
		return m, nil
	case viewServiceList:
		if len(m.filteredServices) == 0 {
			return m, nil
		}
		m.syncTableSelection()
		m.activeService = m.services[m.selectedService]
		m.setServiceActionTable()
		m.actionOpen = true
		m.action.SetCursor(0)
		m.action.Focus()
		m.table.Blur()
		m.status = "Select service action"
		return m, nil
	case viewTaskList:
		if len(m.tasks) == 0 {
			return m, nil
		}
		m.syncTableSelection()
		m.stopLog()
		m.activeTask = m.tasks[m.selectedTask]
		m.taskLogOpen = true
		m.logs = nil
		m.logID++
		m.logTitle = fmt.Sprintf("Task Logs: %s  Esc close", m.activeTask.Name)
		m.logStatus = fmt.Sprintf("%d tasks for %s", len(m.tasks), m.activeService.Name)
		m.resizeComponents()
		m.updateLogView()
		s := startLog(m.logID, []string{"service", "logs", "--tail", "100", "-f", m.activeTask.ID})
		s.stripPrefix = true
		m.logSession = s
		m.status = "Following service task logs"
		return m, waitLogLine(s.id, s.lines)
	}
	return m, nil
}

func (m model) closeTaskLogModal() (tea.Model, tea.Cmd) {
	m.stopLog()
	m.taskLogOpen = false
	m.logs = nil
	m.updateLogView()
	m.status = m.logStatus
	m.applyFocus()
	return m, nil
}

func (m model) showNavSection() (model, tea.Cmd) {
	m.stopLog()
	m.logs = nil
	m.taskLogOpen = false
	m.actionOpen = false
	m.inputOpen = false
	m.confirmOpen = false
	m.serviceRemoveConfirmOpen = false
	m.serviceInspect = serviceInspectState{}
	m.inspectInput = serviceInspectInputState{}
	m.input.Blur()
	if m.navSection == sectionContainers {
		m.mode = viewContainerList
		m.syncFilterInput()
		m.status = "Loading containers"
		m.setContainerTable()
		return m, loadContainers()
	}
	m.mode = viewServiceList
	m.syncFilterInput()
	m.status = "Loading services"
	m.setServiceTable()
	return m, loadServices()
}

func (m model) goBack() (tea.Model, tea.Cmd) {
	if m.inputOpen {
		m.inputOpen = false
		m.input.Blur()
		m.applyFocus()
		return m, nil
	}
	if m.actionOpen {
		m.actionOpen = false
		m.applyFocus()
		return m, nil
	}
	switch m.mode {
	case viewContainerDetail:
		m.stopLog()
		m.mode = viewContainerList
		m.syncFilterInput()
		m.setContainerTable()
		m.status = fmt.Sprintf("%d containers", len(m.containers))
	case viewTaskLog:
		m.stopLog()
		m.mode = viewTaskList
		m.setTaskTable()
		m.status = fmt.Sprintf("%d tasks for %s", len(m.tasks), m.activeService.Name)
	case viewTaskList:
		m.mode = viewServiceList
		m.syncFilterInput()
		m.setServiceTable()
		m.status = fmt.Sprintf("%d services", len(m.services))
	case viewInspect:
		if m.inspectFrom == sectionServices {
			if m.serviceInspect.active && m.serviceInspect.dirty {
				m.confirmOpen = true
				m.confirmExit = true
				return m, nil
			}
			m.mode = viewServiceList
			m.syncFilterInput()
			m.setServiceTable()
			m.status = fmt.Sprintf("%d services", len(m.services))
			m.serviceInspect = serviceInspectState{}
			return m, nil
		}
		m.mode = viewContainerList
		m.syncFilterInput()
		m.setContainerTable()
		m.status = fmt.Sprintf("%d containers", len(m.containers))
	}
	return m, nil
}

func (m model) refresh() (tea.Model, tea.Cmd) {
	switch m.mode {
	case viewContainerList:
		m.status = "Loading containers"
		return m, loadContainers()
	case viewContainerDetail:
		return m, loadStats(m.statsID, m.activeContainer.ID)
	case viewServiceList:
		m.status = "Loading services"
		return m, loadServices()
	case viewTaskList:
		m.status = "Loading service tasks"
		return m, loadTasks(m.activeService.ID)
	}
	return m, nil
}

func (m *model) applyFocus() {
	if m.inputOpen {
		m.table.Blur()
		m.action.Blur()
		return
	}
	if m.confirmOpen {
		m.table.Blur()
		m.action.Blur()
		return
	}
	if m.serviceRemoveConfirmOpen {
		m.table.Blur()
		m.action.Blur()
		return
	}
	if m.actionOpen {
		m.table.Blur()
		m.action.Focus()
		return
	}
	if m.filter.Focused() {
		m.table.Blur()
		return
	}
	if m.mode == viewInspect {
		m.table.Blur()
		return
	}
	if m.focus == focusMain {
		m.table.Focus()
		return
	}
	m.table.Blur()
}

func (m *model) syncTableSelection() {
	switch m.mode {
	case viewContainerList:
		cursor := m.table.Cursor()
		if cursor >= 0 && cursor < len(m.filteredContainers) {
			m.selectedContainer = m.filteredContainers[cursor]
		}
	case viewServiceList:
		cursor := m.table.Cursor()
		if cursor >= 0 && cursor < len(m.filteredServices) {
			m.selectedService = m.filteredServices[cursor]
		}
	case viewTaskList:
		m.selectedTask = clamp(m.table.Cursor(), len(m.tasks))
	}
}

func (m *model) updateInfoTable() {
	switch m.mode {
	case viewContainerDetail:
		m.infoTable.SetColumns(infoColumns(m.rightWidth()))
		m.infoTable.SetRows([]btable.Row{
			{"Name", m.activeContainer.Name},
			{"Image", m.activeContainer.Image},
			{"CPU", m.cpu},
			{"Memory", m.memory},
		})
	case viewTaskLog:
		m.infoTable.SetColumns(infoColumns(m.rightWidth()))
		m.infoTable.SetRows([]btable.Row{
			{"Service", m.activeService.Name},
			{"Task", m.activeTask.Name},
			{"Image", m.activeTask.Image},
			{"State", m.activeTask.CurrentState},
		})
	}
	m.infoTable.Blur()
	m.resizeComponents()
}

func (m *model) updateLogView() {
	m.logView.SetContent(strings.Join(m.logs, "\n"))
	m.logView.GotoBottom()
}

func (m *model) resizeComponents() {
	if m.width <= 0 || m.height <= 0 {
		return
	}
	panelW := m.rightWidth()
	mainContentW := max(1, panelW-2)
	mainContentH := max(1, m.mainPanelHeight()-2)
	listContentW := max(1, mainContentW-listPanelPadding)
	m.nav.SetDelegate(component.NewNavDelegate(max(1, panelW-2)))
	m.nav.SetSize(max(1, panelW-2), max(1, len(m.nav.Items())))

	switch m.mode {
	case viewContainerList:
		m.table.SetColumns(containerColumns(listContentW))
		m.table.SetWidth(listContentW)
		m.table.SetHeight(max(1, mainContentH-2))
		m.filter.Width = max(1, listContentW-8)
	case viewServiceList:
		m.table.SetColumns(serviceColumns(listContentW))
		m.table.SetWidth(listContentW)
		m.table.SetHeight(max(1, mainContentH-2))
		m.filter.Width = max(1, listContentW-8)
	case viewTaskList:
		m.table.SetColumns(taskColumns(listContentW))
		m.table.SetWidth(listContentW)
		m.table.SetHeight(max(1, mainContentH-1))
	case viewContainerDetail, viewTaskLog:
		infoH := min(6, max(1, mainContentH/3))
		m.infoTable.SetColumns(infoColumns(mainContentW))
		m.infoTable.SetWidth(mainContentW)
		m.infoTable.SetHeight(infoH)
		m.logView.Width = mainContentW
		m.logView.Height = max(1, mainContentH-infoH-2)
	case viewInspect:
		m.inspect.Width = max(1, mainContentW-listPanelPadding)
		m.inspect.Height = max(1, mainContentH-1)
	}
	if m.taskLogOpen {
		modalW, modalH := m.taskLogModalSize()
		m.logView.Width = max(1, modalW-6)
		m.logView.Height = max(1, modalH-4)
	}
	component.ResizeActionTable(&m.action, serviceActionPopoverWidth)
	m.input.Width = serviceInputModalWidth - 8
}

func (m model) rightWidth() int {
	w := m.width
	if w <= 0 {
		w = 100
	}
	return w
}

func (m model) bodyHeight() int {
	h := m.height
	if h <= 0 {
		h = 30
	}
	return max(1, h-appTitleHeight-operationHintHeight)
}

func (m model) mainPanelHeight() int {
	return max(3, m.bodyHeight()-navPanelHeight)
}

func containerColumns(width int) []btable.Column {
	nameW := max(10, width/10)
	imageW := max(18, width/5)
	commandW := max(16, width/6)
	createdW := max(10, width/12)
	statusW := max(10, width/10)
	return []btable.Column{
		{Title: "ID", Width: 12},
		{Title: "Name", Width: nameW},
		{Title: "Image", Width: imageW},
		{Title: "Command", Width: commandW},
		{Title: "Created", Width: createdW},
		{Title: "Status", Width: statusW},
		{Title: "Ports", Width: max(12, width-12-nameW-imageW-commandW-createdW-statusW)},
	}
}

func serviceColumns(width int) []btable.Column {
	return []btable.Column{
		{Title: "ID", Width: 12},
		{Title: "Name", Width: 28},
		{Title: "Image", Width: 36},
		{Title: "Replicas", Width: max(8, width-82)},
	}
}

func taskColumns(width int) []btable.Column {
	return []btable.Column{
		{Title: "ID", Width: 12},
		{Title: "Name", Width: 32},
		{Title: "Image", Width: 30},
		{Title: "State", Width: 24},
		{Title: "Node", Width: max(10, width-106)},
	}
}

func infoColumns(width int) []btable.Column {
	return []btable.Column{
		{Title: "Field", Width: 12},
		{Title: "Value", Width: max(20, width-16)},
	}
}

func (m model) View() string {
	h := m.height
	if h <= 0 {
		h = 30
	}
	navBorderColor := "63"
	mainBorderColor := "63"
	if m.focus == focusNav {
		navBorderColor = "39"
	} else {
		mainBorderColor = "39"
	}

	nav := component.TitledPanelActive("Navigation", "81", navBorderColor, m.rightWidth(), navPanelHeight, m.navView(), m.focus == focusNav)

	rightTitle, rightTitleColor := m.mainComponentTitle()
	main := component.TitledPanelActive(rightTitle, rightTitleColor, mainBorderColor, m.rightWidth(), m.mainPanelHeight(), m.mainComponentView(), m.focus == focusMain)

	base := component.AppFrame(m.width, nav, main)
	if m.taskLogOpen {
		return component.Overlay(base, m.taskLogModalView(), m.width, h)
	}
	if m.confirmOpen {
		return component.Overlay(base, m.serviceInspectConfirmView(), m.width, h)
	}
	if m.serviceRemoveConfirmOpen {
		return component.Overlay(base, m.serviceRemoveConfirmView(), m.width, h)
	}
	if m.inputOpen {
		return component.Overlay(base, m.serviceInputModalView(), m.width, h)
	}
	if m.actionOpen {
		x, y := m.serviceActionPopoverPosition()
		if m.mode == viewInspect && m.serviceInspect.active {
			x, y = m.serviceInspectPopoverPosition()
		}
		return component.OverlayAt(base, m.serviceActionPopoverView(), m.width, h, x, y)
	}
	return base
}

func (m model) navView() string {
	items := m.nav.Items()
	titles := make([]string, 0, len(items))
	for _, item := range items {
		nav, ok := item.(navItem)
		if !ok {
			continue
		}
		titles = append(titles, nav.title)
	}
	return component.NavBar(titles, m.nav.Index())
}

func (m model) serviceInputModalView() string {
	title := fmt.Sprintf("Scale service: %s", m.activeService.Name)
	if m.inspectInput.active {
		title = m.serviceInspectInputTitle()
	} else if m.inputAction == serviceActionUpdateImage {
		title = fmt.Sprintf("Update image: %s", m.activeService.Name)
	}
	return component.InputModal(serviceInputModalWidth, title, m.input.View())
}

func (m model) mainComponentTitle() (string, string) {
	switch m.mode {
	case viewContainerList:
		return "Containers", "39"
	case viewServiceList:
		return "Services", "170"
	case viewTaskList:
		return fmt.Sprintf("Tasks for service: %s", m.activeService.Name), "42"
	case viewContainerDetail:
		return fmt.Sprintf("Container: %s", m.activeContainer.Name), "214"
	case viewTaskLog:
		return fmt.Sprintf("Task: %s", m.activeTask.Name), "42"
	case viewInspect:
		return m.inspectTitle, "214"
	default:
		return "", "39"
	}
}

func (m model) mainComponentView() string {
	switch m.mode {
	case viewContainerList:
		return component.FilterTableView(component.FilterTable{
			Filter:  m.filter.View(),
			Table:   m.table.View(),
			Status:  m.status,
			Padding: listPanelPadding,
		})
	case viewServiceList:
		return component.FilterTableView(component.FilterTable{
			Filter:  m.filter.View(),
			Table:   m.table.View(),
			Status:  m.status,
			Padding: listPanelPadding,
		})
	case viewTaskList:
		return component.FilterTableView(component.FilterTable{
			Table:   m.table.View(),
			Status:  m.status,
			Padding: listPanelPadding,
		})
	case viewContainerDetail:
		return component.LogPanel(m.infoTable.View(), m.logView.View(), m.status)
	case viewTaskLog:
		return component.LogPanel(m.infoTable.View(), m.logView.View(), m.status)
	case viewInspect:
		return component.InspectPanel(m.inspect.View(), m.status, listPanelPadding)
	default:
		return m.status
	}
}

func (m *model) refreshServiceInspect() {
	if !m.serviceInspect.active {
		return
	}
	m.serviceInspect.rebuildRows()
	m.inspect.SetContent(m.serviceInspect.content())
	row := m.serviceInspect.selectedRow()
	if row.line < m.inspect.YOffset {
		m.inspect.SetYOffset(row.line)
	}
	if row.line >= m.inspect.YOffset+m.inspect.Height {
		m.inspect.SetYOffset(row.line - m.inspect.Height + 1)
	}
}

func (m model) updateServiceInspectCursor(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch key.String() {
	case "up", "k":
		m.serviceInspect.cursor = clamp(m.serviceInspect.cursor-1, len(m.serviceInspect.rows))
	case "down", "j":
		m.serviceInspect.cursor = clamp(m.serviceInspect.cursor+1, len(m.serviceInspect.rows))
	default:
		var cmd tea.Cmd
		m.inspect, cmd = m.inspect.Update(msg)
		return m, cmd
	}
	m.refreshServiceInspect()
	return m, nil
}

func (m model) openServiceInspectSelection() (tea.Model, tea.Cmd) {
	row := m.serviceInspect.selectedRow()
	switch row.kind {
	case serviceInspectRowAddEnv, serviceInspectRowAddMount, serviceInspectRowAddConstraint:
		return m.openServiceInspectInput(row, true)
	case serviceInspectRowSave:
		if !m.serviceInspect.dirty {
			m.status = "No staged service changes"
			return m, nil
		}
		m.confirmOpen = true
		m.confirmExit = false
		return m, nil
	case serviceInspectRowImage, serviceInspectRowReplicas:
		setActionRows(&m.action, []btable.Row{{"update"}})
	case serviceInspectRowEnv, serviceInspectRowMount, serviceInspectRowConstraint:
		setActionRows(&m.action, []btable.Row{{"update"}, {"remove"}})
	default:
		return m, nil
	}
	m.actionOpen = true
	m.action.SetCursor(0)
	m.action.Focus()
	m.status = "Select inspect action"
	return m, nil
}

func (m model) openServiceInspectAction() (tea.Model, tea.Cmd) {
	row := m.serviceInspect.selectedRow()
	action := "update"
	if m.action.Cursor() == 1 {
		action = "remove"
	}
	m.actionOpen = false
	if action == "update" {
		return m.openServiceInspectInput(row, false)
	}
	switch row.kind {
	case serviceInspectRowEnv:
		m.serviceInspect.removeEnv(row.index)
	case serviceInspectRowMount:
		m.serviceInspect.removeMount(row.index)
	case serviceInspectRowConstraint:
		m.serviceInspect.removeConstraint(row.index)
	}
	m.refreshServiceInspect()
	m.status = "Service inspect change staged"
	return m, nil
}

func (m model) openServiceInspectInput(row serviceInspectRow, add bool) (tea.Model, tea.Cmd) {
	m.inspectInput = serviceInspectInputState{active: true, kind: row.kind, index: row.index, add: add}
	m.inputOpen = true
	m.input.Reset()
	switch row.kind {
	case serviceInspectRowImage:
		m.input.Prompt = "Image: "
		m.input.Placeholder = m.serviceInspect.image()
	case serviceInspectRowReplicas:
		m.input.Prompt = "Replicas: "
		m.input.Placeholder = serviceReplicas(m.serviceInspect.service)
	case serviceInspectRowEnv, serviceInspectRowAddEnv:
		m.input.Prompt = "Env: "
		m.input.Placeholder = "KEY=VALUE"
		if !add && row.index >= 0 {
			m.input.SetValue(m.serviceInspect.env[row.index])
		}
	case serviceInspectRowMount, serviceInspectRowAddMount:
		m.input.Prompt = "Mount: "
		m.input.Placeholder = "type=bind,source=/host,target=/data,readonly=true"
	case serviceInspectRowConstraint, serviceInspectRowAddConstraint:
		m.input.Prompt = "Cons: "
		m.input.Placeholder = "node.labels.disk == ssd"
		if !add && row.index >= 0 {
			m.input.SetValue(m.serviceInspect.constraints[row.index])
		}
	}
	cmd := m.input.Focus()
	m.applyFocus()
	return m, cmd
}

func (m model) serviceInspectInputTitle() string {
	switch m.inspectInput.kind {
	case serviceInspectRowImage:
		return "Update image"
	case serviceInspectRowReplicas:
		return "Update replicas"
	case serviceInspectRowEnv:
		return "Update env"
	case serviceInspectRowAddEnv:
		return "Add env"
	case serviceInspectRowMount:
		return "Update mount"
	case serviceInspectRowAddMount:
		return "Add mount"
	case serviceInspectRowConstraint:
		return "Update constraint"
	case serviceInspectRowAddConstraint:
		return "Add constraint"
	default:
		return "Update service"
	}
}

func (m model) submitServiceInspectInput() (tea.Model, tea.Cmd) {
	value := strings.TrimSpace(m.input.Value())
	if value == "" {
		return m, nil
	}
	m.inputOpen = false
	m.inspectInput.active = false
	m.input.Blur()
	serviceID := m.serviceInspect.service.ID
	switch m.inspectInput.kind {
	case serviceInspectRowImage:
		m.status = "Updating service image"
		return m, updateServiceInspectImage(serviceID, value)
	case serviceInspectRowReplicas:
		m.status = "Updating service replicas"
		return m, updateServiceInspectReplicas(serviceID, value)
	case serviceInspectRowEnv:
		m.serviceInspect.setEnv(m.inspectInput.index, value)
	case serviceInspectRowAddEnv:
		m.serviceInspect.addEnv(value)
	case serviceInspectRowMount:
		mount, err := parseMountSpec(value)
		if err != nil {
			m.status = err.Error()
			m.applyFocus()
			return m, nil
		}
		m.serviceInspect.setMount(m.inspectInput.index, mount)
	case serviceInspectRowAddMount:
		mount, err := parseMountSpec(value)
		if err != nil {
			m.status = err.Error()
			m.applyFocus()
			return m, nil
		}
		m.serviceInspect.addMount(mount)
	case serviceInspectRowConstraint:
		m.serviceInspect.setConstraint(m.inspectInput.index, value)
	case serviceInspectRowAddConstraint:
		m.serviceInspect.addConstraint(value)
	}
	m.refreshServiceInspect()
	m.status = "Service inspect change staged"
	m.applyFocus()
	return m, nil
}

func (m model) confirmServiceInspectChanges() (tea.Model, tea.Cmd) {
	m.confirmOpen = false
	if !m.serviceInspect.dirty {
		return m, nil
	}
	serviceID := m.serviceInspect.service.ID
	cmd := updateServiceInspectStaged(serviceID, m.serviceInspect.env, m.serviceInspect.mounts, m.serviceInspect.constraints, serviceID)
	m.status = "Updating service"
	if m.confirmExit {
		cmd = updateServiceInspectStaged(serviceID, m.serviceInspect.env, m.serviceInspect.mounts, m.serviceInspect.constraints, "")
		m.confirmExit = false
		m.mode = viewServiceList
		m.syncFilterInput()
		m.setServiceTable()
	}
	return m, cmd
}

func (m model) confirmServiceRemove() (tea.Model, tea.Cmd) {
	m.serviceRemoveConfirmOpen = false
	m.status = "Removing service"
	return m, removeService(m.activeService.ID)
}

func (m model) serviceActionPopoverView() string {
	return component.ActionPopover(serviceActionPopoverWidth, m.action.View())
}

func (m model) serviceActionPopoverPosition() (int, int) {
	return 4, appTitleHeight + operationHintHeight + navPanelHeight + 4 + m.table.Cursor()
}

func (m model) serviceInspectPopoverPosition() (int, int) {
	row := m.serviceInspect.selectedRow()
	return 4, appTitleHeight + operationHintHeight + navPanelHeight + 2 + row.line - m.inspect.YOffset
}

func (m model) serviceInspectConfirmView() string {
	body := []string(nil)
	if len(m.serviceInspect.changes) == 0 {
		body = append(body, "  (no changes)")
	} else {
		for _, change := range m.serviceInspect.changes {
			body = append(body, "  "+change)
		}
	}
	return component.ConfirmDialog("Save staged service changes?", body, component.ConfirmFooter{
		Confirm: "Enter confirm",
		Cancel:  "Esc cancel",
	})
}

func (m model) serviceRemoveConfirmView() string {
	return component.ConfirmDialog("Remove service?", []string{
		"  " + m.activeService.Name,
	}, component.ConfirmFooter{
		Confirm: "Enter confirm",
		Cancel:  "Esc cancel",
	})
}

func (m model) taskLogModalView() string {
	modalW, modalH := m.taskLogModalSize()
	title := m.logTitle
	if title == "" {
		title = fmt.Sprintf("Task Logs: %s  Esc close", m.activeTask.Name)
	}
	return component.LogModal(title, modalW, modalH, m.logView.View())
}

func (m model) taskLogModalSize() (int, int) {
	w := m.width
	h := m.height
	if w <= 0 {
		w = 100
	}
	if h <= 0 {
		h = 30
	}
	return min(taskLogModalWidth, max(20, w-4)), min(taskLogModalHeight, max(8, h-4))
}

func (m *model) stopLog() {
	if m.logSession == nil {
		return
	}
	m.logSession.cancel()
	m.logSession = nil
}
