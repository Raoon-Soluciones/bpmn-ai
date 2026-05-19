package bpmn

import (
	"encoding/xml"
	"fmt"
	"os"
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

	// Gateways
	DefaultFlowID string `xml:"default,attr"`

	// Activities
	Assignee       string `xml:"assignee,attr"`
	CandidateUsers string `xml:"candidateUsers,attr"`
	CandidateGroups string `xml:"candidateGroups,attr"`
	Duration       string `xml:"duration,attr"`

	// Script task
	ScriptType string `xml:"scriptType,attr"`
	ScriptBody string `xml:"scriptBody,attr"`

	// Flows
	SourceRef string `xml:"sourceRef,attr"`
	TargetRef string `xml:"targetRef,attr"`
	Condition string `xml:"conditionExpression,attr"`
	IsDefault bool   `xml:"isDefault,attr"`

	// Extension data
	ExtensionElements *extensionElements `xml:"extensionElements"`
}

type eventDefinition struct {
	XMLName     xml.Name
	TimerType   string `xml:"timeDuration"`
	TimerDate   string `xml:"timeDate"`
	TimerCycle  string `xml:"timeCycle"`
	MessageRef  string `xml:"messageRef,attr"`
}

type extensionElements struct {
	Values []extensionValue `xml:",any"`
}

type extensionValue struct {
	XMLName xml.Name
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
	var defs definitions
	if err := xml.Unmarshal(data, &defs); err != nil {
		return nil, fmt.Errorf("unmarshal bpmn xml: %w", err)
	}

	if len(defs.Processes) == 0 {
		return nil, fmt.Errorf("no process found in bpmn definition")
	}

	// Parse the first process (for now)
	proc := defs.Processes[0]
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
			result.Elements[elem.ID] = elem
			if fe.XMLName.Local == "startEvent" && result.StartEventID == "" {
				result.StartEventID = elem.ID
			}

		case "exclusiveGateway", "parallelGateway",
			"inclusiveGateway", "eventBasedGateway":
			elem := p.parseGateway(fe)
			result.Elements[elem.ID] = elem

		case "userTask", "scriptTask", "serviceTask":
			elem := p.parseActivity(fe)
			result.Elements[elem.ID] = elem

		case "sequenceFlow":
			flow := p.parseFlow(fe)
			result.Flows[flow.ID] = flow
		}
	}

	// Wire incoming/outgoing flows
	p.wireFlows(result)

	return result, nil
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
		elem.Type = ElementTypeMessageCatch
	case "intermediateThrowEvent":
		elem.Type = ElementTypeMessageThrow
	case "boundaryEvent":
		elem.Type = ElementTypeTimerEvent
	}

	// Parse event definitions
	for _, ed := range fe.EventDefinitions {
		switch ed.XMLName.Local {
		case "timerEventDefinition":
			elem.EventDefinition.Type = EventTypeTimer
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
		}
	}

	// Check for terminate event definition
	for _, ed := range fe.EventDefinitions {
		if ed.XMLName.Local == "terminateEventDefinition" {
			elem.Type = ElementTypeTerminateEvent
			elem.EventDefinition.Type = EventTypeTerminate
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

	// Parse extension data for conditions
	if fe.ExtensionElements != nil {
		for _, ext := range fe.ExtensionElements.Values {
			elem.ExtensionData[ext.XMLName.Local] = ext.Value
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
	}

	elem.Assignee = fe.Assignee
	elem.Duration = fe.Duration

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
	return Flow{
		ID:        fe.ID,
		Name:      fe.Name,
		SourceRef: fe.SourceRef,
		TargetRef: fe.TargetRef,
		Condition: fe.Condition,
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
