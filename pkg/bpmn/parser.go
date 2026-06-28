package bpmn

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"os"
	"strings"
)

const (
	// MaxXMLSize is the maximum allowed BPMN XML document size (10MB).
	MaxXMLSize = 10 << 20
)

// BPMN definitions XML structure.
type definitions struct {
	XMLName   xml.Name  `xml:"definitions"`
	Namespace string    `xml:"xmlns,attr"`
	TargetNS  string    `xml:"targetNamespace,attr"`
	Processes []process `xml:"process"`
	Messages  []message `xml:"message"`
}

type process struct {
	ID          string        `xml:"id,attr"`
	Name        string        `xml:"name,attr"`
	FlowElements []flowElement `xml:",any"`
}

type flowElement struct {
	XMLName xml.Name
	ID      string `xml:"id,attr"`
	Name    string `xml:"name,attr"`

	// Events
	EventDefinitions []eventDefinition `xml:",any"`
	AttachedToRef    string            `xml:"attachedToRef,attr"`
	CancelActivity   string            `xml:"cancelActivity,attr"`

	// Gateways
	DefaultFlowID    string `xml:"default,attr"`
	GatewayDirection string `xml:"gatewayDirection,attr"`

	// Activities
	Assignee       string `xml:"assignee,attr"`
	CandidateUsers string `xml:"candidateUsers,attr"`
	CandidateGroups string `xml:"candidateGroups,attr"`
	Duration       string `xml:"duration,attr"`

	// Script task
	ScriptType string `xml:"scriptType,attr"`
	ScriptBody string `xml:"scriptBody,attr"`

	// Flows / Relationships
	SourceRef string `xml:"sourceRef,attr"`
	TargetRef string `xml:"targetRef,attr"`
	Condition string `xml:"conditionExpression,attr"`
	IsDefault bool   `xml:"isDefault,attr"`

	// Incoming/Outgoing child elements (used by some BPMN modellers)
	Incoming []string `xml:"incoming"`
	Outgoing []string `xml:"outgoing"`

	// Condition expression as child element (used by e.g. Camunda Modeler)
	ConditionElement string `xml:"conditionExpression"`

	// Call Activity
	CalledElement string `xml:"calledElement,attr"`

	// Extension data
	ExtensionElements *extensionElements `xml:"extensionElements"`

	// Raw inner XML for sub-process parsing
	InnerXML string `xml:",innerxml"`
}

type eventDefinition struct {
	XMLName     xml.Name
	TimerType   string `xml:"timeDuration"`
	TimerDate   string `xml:"timeDate"`
	TimerCycle  string `xml:"timeCycle"`
	MessageRef  string `xml:"messageRef,attr"`
	ErrorRef    string `xml:"errorRef,attr"`
	SignalRef   string `xml:"signalRef,attr"`
}

type extensionElements struct {
	Values []extensionValue `xml:",any"`
}

type extensionValue struct {
	XMLName xml.Name
	Flow    string `xml:"flow,attr"`
	Value   string `xml:",chardata"`
}

type message struct {
	ID   string `xml:"id,attr"`
	Name string `xml:"name,attr"`
}

// Parser parses BPMN 2.0 XML files into Process models.
type Parser struct{}

// NewParser creates a new BPMN parser.
func NewParser() *Parser {
	return &Parser{}
}

// ParseFile reads and parses a BPMN XML file.
func (p *Parser) ParseFile(path string) (*Process, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read bpmn file: %w", err)
	}
	return p.Parse(data)
}

// Parse parses BPMN XML data into a Process model.
func (p *Parser) Parse(data []byte) (*Process, error) {
	if len(data) > MaxXMLSize {
		return nil, fmt.Errorf("xml document exceeds maximum size of %d bytes", MaxXMLSize)
	}

	decoder := xml.NewDecoder(bytes.NewReader(data))
	decoder.Strict = true
	decoder.Entity = nil

	var defs definitions
	if err := decoder.Decode(&defs); err != nil {
		return nil, fmt.Errorf("unmarshal bpmn xml: %w", err)
	}

	if len(defs.Processes) == 0 {
		return nil, fmt.Errorf("no process found in bpmn definition")
	}

	// Parse the first process (for now)
	proc := defs.Processes[0]
	if len(proc.FlowElements) > 500 {
		return nil, fmt.Errorf("process exceeds maximum of 500 elements")
	}
	result := &Process{
		ID:              proc.ID,
		Name:            proc.Name,
		TargetNamespace: defs.TargetNS,
		Elements:        make(map[string]Element),
		Flows:           make(map[string]Flow),
	}

	for _, fe := range proc.FlowElements {
		switch fe.XMLName.Local {
		case "startEvent", "endEvent", "intermediateCatchEvent",
			"intermediateThrowEvent", "boundaryEvent":
			elem := p.parseEvent(fe)
			elem.Incoming = fe.Incoming
			elem.Outgoing = fe.Outgoing
			result.Elements[elem.ID] = elem
			if fe.XMLName.Local == "startEvent" && result.StartEventID == "" {
				result.StartEventID = elem.ID
			}

		case "exclusiveGateway", "parallelGateway",
			"inclusiveGateway", "eventBasedGateway":
			elem := p.parseGateway(fe)
			elem.Incoming = fe.Incoming
			elem.Outgoing = fe.Outgoing
			result.Elements[elem.ID] = elem

		case "userTask", "scriptTask", "serviceTask", "task", "callActivity":
			elem := p.parseActivity(fe)
			elem.Incoming = fe.Incoming
			elem.Outgoing = fe.Outgoing
			result.Elements[elem.ID] = elem

		case "subProcess":
			elem := p.flattenSubProcess(fe, result)
			result.Elements[fe.ID] = elem

		case "sequenceFlow":
			flow := p.parseFlow(fe)
			result.Flows[flow.ID] = flow

			// Create an Element record so the flow can be executed as a step
			extData := map[string]string{
				"sourceRef": flow.SourceRef,
				"targetRef": flow.TargetRef,
			}
			if flow.Condition != "" {
				extData["conditionExpression"] = flow.Condition
			}
			if flow.IsDefault {
				extData["isDefault"] = "true"
			}
			result.Elements[flow.ID] = Element{
				ID:            flow.ID,
				Name:          flow.Name,
				Type:          ElementTypeSequenceFlow,
				ExtensionData: extData,
			}
		}
	}

	// Wire incoming/outgoing flows
	p.wireFlows(result)

	// Add synthetic flows so sequence flow elements route to their targets
	for id, elem := range result.Elements {
		if elem.Type == ElementTypeSequenceFlow {
			if flow, ok := result.Flows[id]; ok {
				syntheticID := id + "_synth"
				result.Flows[syntheticID] = Flow{
					ID:        syntheticID,
					SourceRef: id,
					TargetRef: flow.TargetRef,
				}
				elem.OutgoingFlows = append(elem.OutgoingFlows, syntheticID)
				result.Elements[id] = elem
			}
		}
	}

	// Configure sub-process exit routing: add exit flows to internal end events
	for _, elem := range result.Elements {
		if elem.SubProcess == nil || elem.SubProcessEnd == "" {
			continue
		}
		endEvent, ok := result.Elements[elem.SubProcessEnd]
		if !ok {
			continue
		}
		// Add original (non-synthetic) outgoing flows of the sub-process
		// to its internal end event for exit routing
		for _, flowID := range elem.OutgoingFlows {
			if !strings.HasSuffix(flowID, "_sp_entry") {
				endEvent.OutgoingFlows = append(endEvent.OutgoingFlows, flowID)
			}
		}
		if endEvent.ExtensionData == nil {
			endEvent.ExtensionData = make(map[string]string)
		}
		exitFlows := strings.Join(endEvent.OutgoingFlows, ",")
		endEvent.ExtensionData["subprocess_exit_flows"] = exitFlows
		result.Elements[elem.SubProcessEnd] = endEvent
	}

	// Populate gateway conditions from flow-level conditionExpression attributes
	for id, elem := range result.Elements {
		if isGatewayElement(elem.Type) {
			for _, flowID := range elem.OutgoingFlows {
				if flow, ok := result.Flows[flowID]; ok && flow.Condition != "" {
					if _, exists := elem.Conditions[flowID]; !exists {
						elem.Conditions[flowID] = flow.Condition
					}
				}
			}
			result.Elements[id] = elem
		}
	}

	return result, nil
}

// parseSubProcessXML parses sub-process inner XML into a Process without
// recursively handling nested sub-processes (to avoid infinite recursion).
func (p *Parser) parseSubProcessXML(fe flowElement) *Process {
	if fe.InnerXML == "" {
		return nil
	}

	// Parse child elements from the inner XML using a token-based approach
	// to avoid recursion through the main Parse() path
	type spWrapper struct {
		FlowElements []flowElement `xml:",any"`
	}
	var wrapper spWrapper
	wrappedXML := fmt.Sprintf("<root>%s</root>", fe.InnerXML)
	if err := xml.Unmarshal([]byte(wrappedXML), &wrapper); err != nil {
		return nil
	}

	if len(wrapper.FlowElements) > 100 {
		return nil
	}

	subProc := &Process{
		ID:       fe.ID,
		Name:     fe.Name,
		Elements: make(map[string]Element),
		Flows:    make(map[string]Flow),
	}

	for _, child := range wrapper.FlowElements {
		switch child.XMLName.Local {
		case "startEvent", "endEvent", "intermediateCatchEvent",
			"intermediateThrowEvent", "boundaryEvent":
			e := p.parseEvent(child)
			e.Incoming = child.Incoming
			e.Outgoing = child.Outgoing
			subProc.Elements[e.ID] = e
			if child.XMLName.Local == "startEvent" && subProc.StartEventID == "" {
				subProc.StartEventID = e.ID
			}

		case "exclusiveGateway", "parallelGateway",
			"inclusiveGateway", "eventBasedGateway":
			e := p.parseGateway(child)
			e.Incoming = child.Incoming
			e.Outgoing = child.Outgoing
			subProc.Elements[e.ID] = e

		case "userTask", "scriptTask", "serviceTask", "task", "callActivity":
			e := p.parseActivity(child)
			e.Incoming = child.Incoming
			e.Outgoing = child.Outgoing
			subProc.Elements[e.ID] = e

		case "sequenceFlow":
			f := p.parseFlow(child)
			subProc.Flows[f.ID] = f
			extData := map[string]string{
				"sourceRef": f.SourceRef,
				"targetRef": f.TargetRef,
			}
			if f.Condition != "" {
				extData["conditionExpression"] = f.Condition
			}
			if f.IsDefault {
				extData["isDefault"] = "true"
			}
			subProc.Elements[f.ID] = Element{
				ID:            f.ID,
				Name:          f.Name,
				Type:          ElementTypeSequenceFlow,
				ExtensionData: extData,
			}
		}
	}

	// Wire flows within the sub-process
	p.wireFlows(subProc)

	// Add synthetic flows for sequence flow elements
	for id, e := range subProc.Elements {
		if e.Type == ElementTypeSequenceFlow {
			if f, ok := subProc.Flows[id]; ok {
				syntheticID := id + "_synth"
				subProc.Flows[syntheticID] = Flow{
					ID:        syntheticID,
					SourceRef: id,
					TargetRef: f.TargetRef,
				}
				e.OutgoingFlows = append(e.OutgoingFlows, syntheticID)
				subProc.Elements[id] = e
			}
		}
	}

	return subProc
}

// flattenSubProcess flattens a sub-process's internal elements and flows
// into the main process with prefixed IDs, and sets up synthetic entry/exit routing.
func (p *Parser) flattenSubProcess(fe flowElement, result *Process) Element {
	elem := Element{
		ID:   fe.ID,
		Name: fe.Name,
		Type: ElementTypeSubProcess,
	}

	subProc := p.parseSubProcessXML(fe)
	if subProc == nil {
		return elem
	}

	elem.SubProcess = subProc
	prefix := fe.ID + "."

	// Find the first end event in the sub-process (using original unprefixed ID)
	var subProcessEndID string
	for id, e := range subProc.Elements {
		if e.Type == ElementTypeEndEvent && subProcessEndID == "" {
			subProcessEndID = prefix + id
		}
	}
	elem.SubProcessEnd = subProcessEndID

	// Flatten elements with prefixed IDs
	for id, e := range subProc.Elements {
		newID := prefix + id
		e.ID = newID
		result.Elements[newID] = e
	}

	// Flatten flows with prefixed source/target refs
	for id, f := range subProc.Flows {
		newID := prefix + id
		f.ID = newID
		f.SourceRef = prefix + f.SourceRef
		f.TargetRef = prefix + f.TargetRef
		result.Flows[newID] = f
	}

	// Create synthetic entry flow: subProcess element → internal start event
	entryFlowID := fe.ID + "_sp_entry"
	startID := prefix + subProc.StartEventID
	result.Flows[entryFlowID] = Flow{
		ID:        entryFlowID,
		SourceRef: fe.ID,
		TargetRef: startID,
	}
	elem.Outgoing = append(elem.Outgoing, entryFlowID)

	return elem
}

func (p *Parser) parseEvent(fe flowElement) Element {
	elem := Element{
		ID:   fe.ID,
		Name: fe.Name,
	}

	switch fe.XMLName.Local {
	case "startEvent":
		elem.Type = ElementTypeStartEvent
	case "endEvent":
		elem.Type = ElementTypeEndEvent
	case "intermediateCatchEvent":
		// Will be overridden by event definition below
		elem.Type = ElementTypeMessageCatch
	case "intermediateThrowEvent":
		// Will be overridden by event definition below
		elem.Type = ElementTypeMessageThrow
	case "boundaryEvent":
		// Will be overridden by event definition below
		elem.Type = ElementTypeTimerEvent
		elem.AttachedToRef = fe.AttachedToRef
		elem.CancelActivity = fe.CancelActivity != "false"
	}

	// Parse event definitions and override element type based on definition
	for _, ed := range fe.EventDefinitions {
		switch ed.XMLName.Local {
		case "timerEventDefinition":
			elem.EventDefinition.Type = EventTypeTimer
			elem.Type = ElementTypeTimerEvent
			if ed.TimerType != "" {
				elem.EventDefinition.TimerType = TimerTypeDuration
				elem.EventDefinition.TimerValue = ed.TimerType
			} else if ed.TimerDate != "" {
				elem.EventDefinition.TimerType = TimerTypeDate
				elem.EventDefinition.TimerValue = ed.TimerDate
			} else if ed.TimerCycle != "" {
				elem.EventDefinition.TimerType = TimerTypeCycle
				elem.EventDefinition.TimerValue = ed.TimerCycle
			}
		case "messageEventDefinition":
			elem.EventDefinition.Type = EventTypeMessage
			elem.EventDefinition.MessageRef = ed.MessageRef
			// Catch events receive messages; throw events send them
			if fe.XMLName.Local == "intermediateCatchEvent" || fe.XMLName.Local == "boundaryEvent" {
				elem.Type = ElementTypeMessageCatch
			} else {
				elem.Type = ElementTypeMessageThrow
			}
		case "signalEventDefinition":
			elem.EventDefinition.Type = EventTypeSignal
			elem.EventDefinition.SignalRef = ed.SignalRef
			if fe.XMLName.Local == "intermediateCatchEvent" || fe.XMLName.Local == "boundaryEvent" || fe.XMLName.Local == "startEvent" {
				elem.Type = ElementTypeSignalCatch
			} else {
				elem.Type = ElementTypeSignalThrow
			}
		case "terminateEventDefinition":
			elem.Type = ElementTypeTerminateEvent
			elem.EventDefinition.Type = EventTypeTerminate
		case "errorEventDefinition":
			elem.EventDefinition.Type = EventTypeError
			elem.EventDefinition.ErrorCode = ed.ErrorRef
			// End / intermediate throw events with error definition throw the error
			if fe.XMLName.Local == "endEvent" || fe.XMLName.Local == "intermediateThrowEvent" {
				elem.Type = ElementTypeErrorEnd
			}
			// Boundary events with error definition catch the error
			if fe.XMLName.Local == "boundaryEvent" {
				elem.Type = ElementTypeErrorCatch
				elem.AttachedToRef = fe.AttachedToRef
			}
			// Start events with error definition catch error (for event sub-processes)
			if fe.XMLName.Local == "startEvent" {
				elem.Type = ElementTypeErrorCatch
			}
		}
	}

	return elem
}

func (p *Parser) parseGateway(fe flowElement) Element {
	elem := Element{
		ID:            fe.ID,
		Name:          fe.Name,
		DefaultFlowID: fe.DefaultFlowID,
		Conditions:    make(map[string]string),
	}

	switch fe.XMLName.Local {
	case "exclusiveGateway":
		elem.Type = ElementTypeExclusiveGateway
		elem.GatewayType = GatewayTypeExclusive
	case "parallelGateway":
		elem.Type = ElementTypeParallelGateway
		elem.GatewayType = GatewayTypeParallel
	case "inclusiveGateway":
		elem.Type = ElementTypeInclusiveGateway
		elem.GatewayType = GatewayTypeInclusive
	case "eventBasedGateway":
		elem.Type = ElementTypeEventBasedGateway
		elem.GatewayType = GatewayTypeEventBased
	}

	// Parse gatewayDirection attribute
	switch GatewayDirection(fe.GatewayDirection) {
	case GatewayDirectionDiverging:
		elem.GatewayDirection = GatewayDirectionDiverging
	case GatewayDirectionConverging:
		elem.GatewayDirection = GatewayDirectionConverging
	case GatewayDirectionMixed:
		elem.GatewayDirection = GatewayDirectionMixed
	}

	// Parse extension data and conditions
	if fe.ExtensionElements != nil {
		elem.ExtensionData = make(map[string]string)
		for _, ext := range fe.ExtensionElements.Values {
			if ext.XMLName.Local == "condition" && ext.Flow != "" {
				elem.Conditions[ext.Flow] = ext.Value
			} else {
				elem.ExtensionData[ext.XMLName.Local] = ext.Value
			}
		}
	}

	return elem
}

func (p *Parser) parseActivity(fe flowElement) Element {
	elem := Element{
		ID:   fe.ID,
		Name: fe.Name,
	}

	switch fe.XMLName.Local {
	case "userTask":
		elem.Type = ElementTypeUserTask
		elem.TaskType = TaskTypeUser
	case "scriptTask":
		elem.Type = ElementTypeScriptTask
		elem.TaskType = TaskTypeScript
	case "serviceTask":
		elem.Type = ElementTypeServiceTask
		elem.TaskType = TaskTypeService
	case "task":
		elem.Type = ElementTypeUserTask
		elem.TaskType = TaskTypeUser
	case "callActivity":
		elem.Type = ElementTypeCallActivity
	}

	elem.Assignee = fe.Assignee
	elem.Duration = fe.Duration
	elem.CalledElement = fe.CalledElement

	if fe.ScriptBody != "" || fe.ScriptType != "" {
		elem.ExtensionData = make(map[string]string)
		if fe.ScriptBody != "" {
			elem.ExtensionData["scriptBody"] = fe.ScriptBody
		}
		if fe.ScriptType != "" {
			elem.ExtensionData["scriptType"] = fe.ScriptType
		}
	}

	if fe.CandidateUsers != "" {
		// Parse comma-separated list
		for _, u := range splitCSV(fe.CandidateUsers) {
			elem.CandidateUsers = append(elem.CandidateUsers, u)
		}
	}
	if fe.CandidateGroups != "" {
		for _, g := range splitCSV(fe.CandidateGroups) {
			elem.CandidateGroups = append(elem.CandidateGroups, g)
		}
	}

	return elem
}

func (p *Parser) parseFlow(fe flowElement) Flow {
	cond := fe.Condition
	if cond == "" && fe.ConditionElement != "" {
		cond = fe.ConditionElement
	}
	return Flow{
		ID:        fe.ID,
		Name:      fe.Name,
		SourceRef: fe.SourceRef,
		TargetRef: fe.TargetRef,
		Condition: cond,
		IsDefault: fe.IsDefault,
	}
}

func (p *Parser) wireFlows(proc *Process) {
	for flowID, flow := range proc.Flows {
		if elem, ok := proc.Elements[flow.SourceRef]; ok {
			elem.OutgoingFlows = append(elem.OutgoingFlows, flowID)
			proc.Elements[flow.SourceRef] = elem
		}
		if elem, ok := proc.Elements[flow.TargetRef]; ok {
			elem.IncomingFlows = append(elem.IncomingFlows, flowID)
			proc.Elements[flow.TargetRef] = elem
		}
	}

	// use incoming/outgoing child elements from elements that weren't wired by flows
	for id, elem := range proc.Elements {
		if len(elem.IncomingFlows) == 0 && len(elem.OutgoingFlows) == 0 {
			for _, incoming := range elem.Incoming {
				if flow, ok := proc.Flows[incoming]; ok {
					if flow.TargetRef == id {
						elem.IncomingFlows = append(elem.IncomingFlows, incoming)
					}
				}
			}
			for _, outgoing := range elem.Outgoing {
				if flow, ok := proc.Flows[outgoing]; ok {
					if flow.SourceRef == id {
						elem.OutgoingFlows = append(elem.OutgoingFlows, outgoing)
					}
				}
			}
			proc.Elements[id] = elem
		}
	}
}

func isGatewayElement(t ElementType) bool {
	return t == ElementTypeExclusiveGateway || t == ElementTypeInclusiveGateway ||
		t == ElementTypeParallelGateway || t == ElementTypeEventBasedGateway
}

func splitCSV(s string) []string {
	var result []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			val := s[start:i]
			if val = trimSpace(val); val != "" {
				result = append(result, val)
			}
			start = i + 1
		}
	}
	if val := trimSpace(s[start:]); val != "" {
		result = append(result, val)
	}
	return result
}

func trimSpace(s string) string {
	if len(s) == 0 {
		return s
	}
	start := 0
	for start < len(s) && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	end := len(s) - 1
	for end >= start && (s[end] == ' ' || s[end] == '\t') {
		end--
	}
	if end < start {
		return ""
	}
	return s[start : end+1]
}
