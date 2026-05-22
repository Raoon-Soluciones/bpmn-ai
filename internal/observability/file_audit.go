package observability

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const auditFilePattern = "audit_%s.log"

type instanceAuditState struct {
	mu           sync.Mutex
	step         int
	elementCount int
	startTime    time.Time
	processName  string
	processID    string
	file         *os.File
}

type FileAuditWriter struct {
	dir       string
	enabled   bool
	logger    *Logger
	instances map[string]*instanceAuditState
	wmu       sync.Mutex
}

func NewFileAuditWriter(dir string, enabled bool, logger *Logger) (*FileAuditWriter, error) {
	if enabled {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("create audit dir: %w", err)
		}
	}
	return &FileAuditWriter{
		dir:       dir,
		enabled:   enabled,
		logger:    logger,
		instances: make(map[string]*instanceAuditState),
	}, nil
}

func (w *FileAuditWriter) getOrCreateState(instanceID string) *instanceAuditState {
	w.wmu.Lock()
	defer w.wmu.Unlock()
	st, ok := w.instances[instanceID]
	if ok && st.file != nil {
		return st
	}
	if ok && st.file == nil {
		path := filepath.Join(w.dir, fmt.Sprintf(auditFilePattern, instanceID))
		f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err == nil {
			st.file = f
		}
		return st
	}
	path := filepath.Join(w.dir, fmt.Sprintf(auditFilePattern, instanceID))
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		st = &instanceAuditState{file: f}
	} else {
		st = &instanceAuditState{}
		if w.logger != nil {
			w.logger.Error("audit: failed to open file", "error", err, "instance", instanceID)
		}
	}
	w.instances[instanceID] = st
	return st
}

func (w *FileAuditWriter) getState(instanceID string) *instanceAuditState {
	w.wmu.Lock()
	defer w.wmu.Unlock()
	return w.instances[instanceID]
}

func (w *FileAuditWriter) removeState(instanceID string) {
	w.wmu.Lock()
	st, ok := w.instances[instanceID]
	if ok {
		if st.file != nil {
			st.file.Close()
		}
		delete(w.instances, instanceID)
	}
	w.wmu.Unlock()
}

func (w *FileAuditWriter) writeToState(instanceID string, text string) {
	if !w.enabled {
		return
	}
	st := w.getOrCreateState(instanceID)
	st.mu.Lock()
	defer st.mu.Unlock()
	w.ensureFileOpen(st, instanceID)
	if st.file != nil {
		writeFile(st.file, text)
	}
}

func writeFile(f *os.File, s string) {
	_, err := f.WriteString(s)
	if err != nil {
		// write errors are non-fatal for audit
	}
}

func (w *FileAuditWriter) HandleEvent(event Event) {
	if !w.enabled {
		return
	}
	if event.Payload == nil {
		return
	}
	instanceID := extractString(event.Payload, "instance_id")
	if instanceID == "" {
		return
	}

	switch event.Type {
	case EventProcessStarted:
		w.handleProcessStarted(instanceID, event.Payload)
	case EventElementExecuted:
		w.handleElementExecuted(instanceID, event.Payload)
	case EventElementError:
		w.handleElementError(instanceID, event.Payload)
	case EventProcessCompleted:
		w.handleProcessCompleted(instanceID, event.Payload, "COMPLETED")
	case EventProcessTerminated:
		w.handleProcessTerminated(instanceID, event.Payload)
	case EventProcessError:
		w.handleProcessError(instanceID, event.Payload)
	case EventTaskClaimed:
		w.handleTaskClaimed(instanceID, event.Payload)
	case EventTaskCompleted:
		w.handleTaskCompleted(instanceID, event.Payload)
	}
}

func (w *FileAuditWriter) handleProcessStarted(instanceID string, payload map[string]any) {
	st := w.getOrCreateState(instanceID)
	st.mu.Lock()
	defer st.mu.Unlock()

	st.startTime = time.Now()
	st.processID = extractString(payload, "process_id")
	st.processName = extractString(payload, "process_name")

	var b strings.Builder
	b.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	b.WriteString(" BPMN Execution Audit\n")
	b.WriteString(fmt.Sprintf(" Process:   %s (%s)\n", st.processName, st.processID))
	b.WriteString(fmt.Sprintf(" Instance:  %s\n", instanceID))
	b.WriteString(fmt.Sprintf(" Started:   %s\n", st.startTime.Format("2006-01-02 15:04:05.000")))
	b.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

	writeFile(st.file, b.String())
}

func (w *FileAuditWriter) ensureFileOpen(st *instanceAuditState, instanceID string) {
	if st.file != nil {
		return
	}
	path := filepath.Join(w.dir, fmt.Sprintf(auditFilePattern, instanceID))
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		st.file = f
	}
}

func (w *FileAuditWriter) handleElementExecuted(instanceID string, payload map[string]any) {
	st := w.getOrCreateState(instanceID)
	st.mu.Lock()
	defer st.mu.Unlock()
	w.ensureFileOpen(st, instanceID)
	st.step++
	st.elementCount++

	elemID := extractString(payload, "element_id")
	elemName := extractString(payload, "element_name")
	elemType := extractString(payload, "element_type")
	action := extractString(payload, "action")
	threadID := extractInt(payload, "thread_id")
	duration := extractInt(payload, "duration_ms")
	fromState := extractString(payload, "from_state")
	toState := extractString(payload, "to_state")

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  %d.  Thread %d  │  %s", st.step, threadID, elemID))
	if elemName != "" {
		b.WriteString(fmt.Sprintf(" \"%s\"", elemName))
	}
	b.WriteString(fmt.Sprintf("  │  %s\n", elemType))
	b.WriteString(fmt.Sprintf("      %s", action))
	if duration > 0 {
		b.WriteString(fmt.Sprintf("  ·  %dms", duration))
	}
	b.WriteString("\n")

	if flowFilters, ok := payload["flow_filters"]; ok {
		if filters, ok := flowFilters.([]any); ok {
			for _, f := range filters {
				if m, ok := f.(map[string]any); ok {
					target := extractString(m, "target_ref")
					if target != "" {
						b.WriteString(fmt.Sprintf("      Flow: → %s\n", target))
					}
				}
			}
		}
	}

	if fromState != "" && toState != "" {
		b.WriteString(fmt.Sprintf("      State: %s → %s\n", fromState, toState))
	}

	if st.file != nil {
		writeFile(st.file, b.String())
	}
}

func (w *FileAuditWriter) handleElementError(instanceID string, payload map[string]any) {
	st := w.getOrCreateState(instanceID)
	st.mu.Lock()
	defer st.mu.Unlock()
	w.ensureFileOpen(st, instanceID)
	st.step++
	st.elementCount++

	elemID := extractString(payload, "element_id")
	elemName := extractString(payload, "element_name")
	elemType := extractString(payload, "element_type")
	errMsg := extractString(payload, "error")
	threadID := extractInt(payload, "thread_id")

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  %d.  Thread %d  │  %s", st.step, threadID, elemID))
	if elemName != "" {
		b.WriteString(fmt.Sprintf(" \"%s\"", elemName))
	}
	b.WriteString(fmt.Sprintf("  │  %s\n", elemType))
	b.WriteString("      ⛔ ERROR\n")
	if errMsg != "" {
		b.WriteString(fmt.Sprintf("      Error: %s\n", errMsg))
	}

	if st.file != nil {
		writeFile(st.file, b.String())
	}
}

func (w *FileAuditWriter) handleProcessCompleted(instanceID string, payload map[string]any, status string) {
	st := w.getState(instanceID)
	if st == nil {
		return
	}
	st.mu.Lock()

	elapsed := time.Since(st.startTime)

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	b.WriteString(fmt.Sprintf(" Result: %s\n", status))
	b.WriteString(fmt.Sprintf(" Duration: %s\n", formatDuration(elapsed)))
	b.WriteString(fmt.Sprintf(" Elements: %d\n", st.elementCount))
	b.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

	if st.file != nil {
		writeFile(st.file, b.String())
		st.file.Close()
		st.file = nil
	}
	st.mu.Unlock()
}

func (w *FileAuditWriter) handleProcessTerminated(instanceID string, payload map[string]any) {
	st := w.getState(instanceID)
	if st == nil {
		return
	}
	st.mu.Lock()

	elapsed := time.Since(st.startTime)

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	b.WriteString(fmt.Sprintf(" Result: TERMINATED\n"))
	if elemID := extractString(payload, "element_id"); elemID != "" {
		b.WriteString(fmt.Sprintf("  Stopped at: %s\n", elemID))
	}
	b.WriteString(fmt.Sprintf(" Duration: %s\n", formatDuration(elapsed)))
	b.WriteString(fmt.Sprintf(" Elements: %d\n", st.elementCount))
	b.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

	if st.file != nil {
		writeFile(st.file, b.String())
		st.file.Close()
		st.file = nil
	}
	st.mu.Unlock()
}

func (w *FileAuditWriter) handleProcessError(instanceID string, payload map[string]any) {
	st := w.getState(instanceID)
	if st == nil {
		return
	}
	st.mu.Lock()

	elapsed := time.Since(st.startTime)
	errMsg := extractString(payload, "error")

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	b.WriteString(" Result: ERROR\n")
	if errMsg != "" {
		b.WriteString(fmt.Sprintf(" Error: %s\n", errMsg))
	}
	b.WriteString(fmt.Sprintf(" Duration: %s\n", formatDuration(elapsed)))
	b.WriteString(fmt.Sprintf(" Elements: %d\n", st.elementCount))
	b.WriteString("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

	if st.file != nil {
		writeFile(st.file, b.String())
		st.file.Close()
		st.file = nil
	}
	st.mu.Unlock()
}

func (w *FileAuditWriter) handleTaskClaimed(instanceID string, payload map[string]any) {
	elemID := extractString(payload, "element_id")
	elemName := extractString(payload, "element_name")
	threadID := extractInt(payload, "thread_id")
	assignee := extractString(payload, "assignee")

	var b strings.Builder
	b.WriteString(fmt.Sprintf("\n  •  Task claimed: %s", elemID))
	if elemName != "" {
		b.WriteString(fmt.Sprintf(" \"%s\"", elemName))
	}
	if assignee != "" {
		b.WriteString(fmt.Sprintf(" by %s", assignee))
	}
	b.WriteString(fmt.Sprintf("  (Thread %d)\n", threadID))

	w.writeToState(instanceID, b.String())
}

func (w *FileAuditWriter) handleTaskCompleted(instanceID string, payload map[string]any) {
	elemID := extractString(payload, "element_id")
	elemName := extractString(payload, "element_name")
	threadID := extractInt(payload, "thread_id")

	var b strings.Builder
	b.WriteString(fmt.Sprintf("\n  •  Task completed: %s", elemID))
	if elemName != "" {
		b.WriteString(fmt.Sprintf(" \"%s\"", elemName))
	}
	b.WriteString(fmt.Sprintf("  (Thread %d)\n", threadID))

	w.writeToState(instanceID, b.String())
}

func (w *FileAuditWriter) Close() error {
	w.wmu.Lock()
	defer w.wmu.Unlock()
	for _, st := range w.instances {
		st.mu.Lock()
		if st.file != nil {
			st.file.Close()
			st.file = nil
		}
		st.mu.Unlock()
	}
	w.instances = make(map[string]*instanceAuditState)
	return nil
}

func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.2fs", d.Seconds())
	}
	return d.Round(time.Second).String()
}

func extractString(payload map[string]any, key string) string {
	if payload == nil {
		return ""
	}
	v, ok := payload[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

func extractInt(payload map[string]any, key string) int {
	if payload == nil {
		return 0
	}
	v, ok := payload[key]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(math.Round(n))
	default:
		return 0
	}
}
