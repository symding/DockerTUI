package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	mounttypes "github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/swarm"
)

type serviceInspectRowKind int

const (
	serviceInspectRowImage serviceInspectRowKind = iota
	serviceInspectRowReplicas
	serviceInspectRowEnv
	serviceInspectRowAddEnv
	serviceInspectRowMount
	serviceInspectRowAddMount
	serviceInspectRowConstraint
	serviceInspectRowAddConstraint
	serviceInspectRowSave
)

type serviceInspectRow struct {
	kind  serviceInspectRowKind
	index int
	line  int
}

type serviceInspectState struct {
	active      bool
	service     swarm.Service
	rows        []serviceInspectRow
	cursor      int
	env         []string
	mounts      []mounttypes.Mount
	constraints []string
	dirty       bool
	changes     []string
}

type serviceInspectInputState struct {
	active bool
	kind   serviceInspectRowKind
	index  int
	add    bool
}

func newServiceInspectState(service swarm.Service) serviceInspectState {
	state := serviceInspectState{
		active:  true,
		service: service,
	}
	if service.Spec.TaskTemplate.ContainerSpec != nil {
		state.env = append([]string(nil), service.Spec.TaskTemplate.ContainerSpec.Env...)
		state.mounts = append([]mounttypes.Mount(nil), service.Spec.TaskTemplate.ContainerSpec.Mounts...)
	}
	if service.Spec.TaskTemplate.Placement != nil {
		state.constraints = append([]string(nil), service.Spec.TaskTemplate.Placement.Constraints...)
	}
	state.rebuildRows()
	return state
}

func (s *serviceInspectState) rebuildRows() {
	s.rows = nil
	s.rows = append(s.rows, serviceInspectRow{kind: serviceInspectRowImage, line: 1})
	s.rows = append(s.rows, serviceInspectRow{kind: serviceInspectRowReplicas, line: 3})
	line := 6
	for i := range s.env {
		s.rows = append(s.rows, serviceInspectRow{kind: serviceInspectRowEnv, index: i, line: line})
		line++
	}
	if len(s.env) == 0 {
		line++
	}
	s.rows = append(s.rows, serviceInspectRow{kind: serviceInspectRowAddEnv, index: -1, line: line})
	line += 3
	for i := range s.mounts {
		s.rows = append(s.rows, serviceInspectRow{kind: serviceInspectRowMount, index: i, line: line})
		line++
	}
	if len(s.mounts) == 0 {
		line++
	}
	s.rows = append(s.rows, serviceInspectRow{kind: serviceInspectRowAddMount, index: -1, line: line})
	line += 3
	for i := range s.constraints {
		s.rows = append(s.rows, serviceInspectRow{kind: serviceInspectRowConstraint, index: i, line: line})
		line++
	}
	if len(s.constraints) == 0 {
		line++
	}
	s.rows = append(s.rows, serviceInspectRow{kind: serviceInspectRowAddConstraint, index: -1, line: line})
	line += 2
	s.rows = append(s.rows, serviceInspectRow{kind: serviceInspectRowSave, index: -1, line: line})
	s.cursor = clamp(s.cursor, len(s.rows))
}

func (s serviceInspectState) selectedRow() serviceInspectRow {
	if len(s.rows) == 0 {
		return serviceInspectRow{}
	}
	return s.rows[clamp(s.cursor, len(s.rows))]
}

func (s serviceInspectState) serviceName() string {
	if s.service.Spec.Name != "" {
		return s.service.Spec.Name
	}
	return s.service.ID
}

func (s serviceInspectState) image() string {
	if s.service.Spec.TaskTemplate.ContainerSpec == nil || s.service.Spec.TaskTemplate.ContainerSpec.Image == "" {
		return "-"
	}
	return s.service.Spec.TaskTemplate.ContainerSpec.Image
}

func (s serviceInspectState) content() string {
	selected := s.selectedRow()
	var b strings.Builder
	writeInspectLine(&b, "Name: "+s.serviceName(), false)
	writeInspectLine(&b, "Image: "+s.image(), selected.kind == serviceInspectRowImage)
	writeInspectLine(&b, "Runtime: "+formatDuration(time.Since(s.service.CreatedAt)), false)
	writeInspectLine(&b, "Replicas: "+serviceReplicas(s.service), selected.kind == serviceInspectRowReplicas)
	b.WriteByte('\n')
	b.WriteString("Environment\n")
	if len(s.env) == 0 {
		b.WriteString("  (none)\n")
	}
	for i, env := range s.env {
		writeInspectLine(&b, "  "+env, selected.kind == serviceInspectRowEnv && selected.index == i)
	}
	writeInspectLine(&b, "  + add env", selected.kind == serviceInspectRowAddEnv)
	b.WriteByte('\n')
	b.WriteString("Mounts\n")
	if len(s.mounts) == 0 {
		b.WriteString("  (none)\n")
	}
	for i, mount := range s.mounts {
		writeInspectLine(&b, "  "+formatServiceMount(mount), selected.kind == serviceInspectRowMount && selected.index == i)
	}
	writeInspectLine(&b, "  + add mount", selected.kind == serviceInspectRowAddMount)
	b.WriteByte('\n')
	b.WriteString("Constraints\n")
	if len(s.constraints) == 0 {
		b.WriteString("  (none)\n")
	}
	for i, constraint := range s.constraints {
		writeInspectLine(&b, "  "+constraint, selected.kind == serviceInspectRowConstraint && selected.index == i)
	}
	writeInspectLine(&b, "  + add cons", selected.kind == serviceInspectRowAddConstraint)
	b.WriteByte('\n')
	label := "Save change"
	if s.dirty {
		label = "Save change *"
	}
	writeInspectLine(&b, label, selected.kind == serviceInspectRowSave)
	return b.String()
}

func writeInspectLine(b *strings.Builder, value string, selected bool) {
	if selected {
		value = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("39")).
			Background(lipgloss.Color("236")).
			Render(value)
	}
	b.WriteString(value)
	b.WriteByte('\n')
}

func formatServiceMount(mount mounttypes.Mount) string {
	source := mount.Source
	if source == "" {
		source = "-"
	}
	mode := "rw"
	if mount.ReadOnly {
		mode = "ro"
	}
	return fmt.Sprintf("%s %s -> %s (%s)", mount.Type, source, mount.Target, mode)
}

func (s *serviceInspectState) addChange(change string) {
	s.dirty = true
	s.changes = append(s.changes, change)
}

func (s *serviceInspectState) setEnv(index int, value string) {
	s.env[index] = value
	s.addChange("Update env: " + value)
}

func (s *serviceInspectState) addEnv(value string) {
	s.env = append(s.env, value)
	s.addChange("Add env: " + value)
	s.rebuildRows()
}

func (s *serviceInspectState) removeEnv(index int) {
	value := s.env[index]
	s.env = append(s.env[:index], s.env[index+1:]...)
	s.addChange("Remove env: " + value)
	s.rebuildRows()
}

func (s *serviceInspectState) setMount(index int, mount mounttypes.Mount) {
	s.mounts[index] = mount
	s.addChange("Update mount: " + formatServiceMount(mount))
}

func (s *serviceInspectState) addMount(mount mounttypes.Mount) {
	s.mounts = append(s.mounts, mount)
	s.addChange("Add mount: " + formatServiceMount(mount))
	s.rebuildRows()
}

func (s *serviceInspectState) removeMount(index int) {
	value := formatServiceMount(s.mounts[index])
	s.mounts = append(s.mounts[:index], s.mounts[index+1:]...)
	s.addChange("Remove mount: " + value)
	s.rebuildRows()
}

func (s *serviceInspectState) setConstraint(index int, value string) {
	s.constraints[index] = value
	s.addChange("Update constraint: " + value)
}

func (s *serviceInspectState) addConstraint(value string) {
	s.constraints = append(s.constraints, value)
	s.addChange("Add constraint: " + value)
	s.rebuildRows()
}

func (s *serviceInspectState) removeConstraint(index int) {
	value := s.constraints[index]
	s.constraints = append(s.constraints[:index], s.constraints[index+1:]...)
	s.addChange("Remove constraint: " + value)
	s.rebuildRows()
}

func parseMountSpec(value string) (mounttypes.Mount, error) {
	var result mounttypes.Mount
	for _, part := range strings.Split(value, ",") {
		key, val, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "type":
			result.Type = mounttypes.Type(strings.TrimSpace(val))
		case "source", "src":
			result.Source = strings.TrimSpace(val)
		case "target", "dst", "destination":
			result.Target = strings.TrimSpace(val)
		case "readonly", "ro":
			result.ReadOnly = parseMountBool(val)
		}
	}
	if result.Type == "" || result.Target == "" {
		return mounttypes.Mount{}, fmt.Errorf("mount requires type and target")
	}
	return result, nil
}

func parseMountBool(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "ro", "readonly":
		return true
	default:
		return false
	}
}
