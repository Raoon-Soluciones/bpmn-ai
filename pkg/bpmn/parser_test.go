package bpmn

import "testing"

func TestParser_ParseFile_NotFound(t *testing.T) {
	p := NewParser()
	_, err := p.ParseFile("nonexistent.bpmn")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestParser_Parse_InvalidXML(t *testing.T) {
	p := NewParser()
	_, err := p.Parse([]byte("not xml"))
	if err == nil {
		t.Fatal("expected error for invalid xml")
	}
}

func TestParser_Parse_Empty(t *testing.T) {
	p := NewParser()
	_, err := p.Parse([]byte(`<?xml version="1.0"?><definitions xmlns="http://www.omg.org/spec/BPMN/20100524/MODEL"/>`))
	if err == nil {
		t.Fatal("expected error for empty definitions")
	}
}

func TestParser_Parse_SimpleSequence(t *testing.T) {
	xml := `<?xml version="1.0" encoding="UTF-8"?>
<definitions xmlns="http://www.omg.org/spec/BPMN/20100524/MODEL" targetNamespace="test">
  <process id="proc-1" name="Simple Process">
    <startEvent id="start-1" name="Start"/>
    <sequenceFlow id="flow-1" sourceRef="start-1" targetRef="end-1"/>
    <endEvent id="end-1" name="End"/>
  </process>
</definitions>`

	p := NewParser()
	proc, err := p.Parse([]byte(xml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if proc.ID != "proc-1" {
		t.Errorf("expected ID proc-1, got %s", proc.ID)
	}
	if proc.Name != "Simple Process" {
		t.Errorf("expected name 'Simple Process', got %s", proc.Name)
	}
	if proc.StartEventID != "start-1" {
		t.Errorf("expected start event start-1, got %s", proc.StartEventID)
	}
	if len(proc.Elements) != 3 {
		t.Errorf("expected 3 elements (start, end, flow), got %d", len(proc.Elements))
	}
	if len(proc.Flows) != 2 {
		t.Errorf("expected 2 flows (1 real + 1 synthetic), got %d", len(proc.Flows))
	}

	start := proc.Elements["start-1"]
	if start.Type != ElementTypeStartEvent {
		t.Errorf("expected startEvent type, got %s", start.Type)
	}
	if len(start.OutgoingFlows) != 1 || start.OutgoingFlows[0] != "flow-1" {
		t.Errorf("expected outgoing flow-1, got %v", start.OutgoingFlows)
	}

	end := proc.Elements["end-1"]
	if end.Type != ElementTypeEndEvent {
		t.Errorf("expected endEvent type, got %s", end.Type)
	}
	if len(end.IncomingFlows) != 1 || end.IncomingFlows[0] != "flow-1" {
		t.Errorf("expected incoming flow-1, got %v", end.IncomingFlows)
	}

	flow := proc.Flows["flow-1"]
	if flow.SourceRef != "start-1" {
		t.Errorf("expected sourceRef start-1, got %s", flow.SourceRef)
	}
	if flow.TargetRef != "end-1" {
		t.Errorf("expected targetRef end-1, got %s", flow.TargetRef)
	}
}

func TestParser_Parse_ExclusiveGatewayConditions(t *testing.T) {
	xml := `<?xml version="1.0" encoding="UTF-8"?>
<definitions xmlns="http://www.omg.org/spec/BPMN/20100524/MODEL" targetNamespace="test">
  <process id="proc-3" name="Condition Process">
    <startEvent id="start-1"/>
    <sequenceFlow id="flow-1" sourceRef="start-1" targetRef="gw-1"/>
    <exclusiveGateway id="gw-1" name="Decision" default="flow-reject">
      <extensionElements>
        <condition flow="flow-approve">amount &lt;= 1000</condition>
      </extensionElements>
    </exclusiveGateway>
    <sequenceFlow id="flow-approve" sourceRef="gw-1" targetRef="end-1"/>
    <sequenceFlow id="flow-reject" sourceRef="gw-1" targetRef="end-2"/>
    <endEvent id="end-1"/>
    <endEvent id="end-2"/>
  </process>
</definitions>`

	p := NewParser()
	proc, err := p.Parse([]byte(xml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	gw := proc.Elements["gw-1"]
	if len(gw.Conditions) != 1 {
		t.Errorf("expected 1 condition, got %d", len(gw.Conditions))
	}
	cond, ok := gw.Conditions["flow-approve"]
	if !ok {
		t.Errorf("expected condition for flow-approve")
	}
	if cond != "amount <= 1000" {
		t.Errorf("expected 'amount <= 1000', got '%s'", cond)
	}
}

func TestParser_Parse_FlowConditionExpression(t *testing.T) {
	xml := `<?xml version="1.0" encoding="UTF-8"?>
<definitions xmlns="http://www.omg.org/spec/BPMN/20100524/MODEL" targetNamespace="test">
  <process id="proc-2" name="Gateway Process">
    <startEvent id="start-1"/>
    <sequenceFlow id="flow-1" sourceRef="start-1" targetRef="gw-1"/>
    <exclusiveGateway id="gw-1" name="Decision" default="flow-3"/>
    <sequenceFlow id="flow-2" sourceRef="gw-1" targetRef="end-1" conditionExpression="${approved}"/>
    <sequenceFlow id="flow-3" sourceRef="gw-1" targetRef="end-2"/>
    <endEvent id="end-1"/>
    <endEvent id="end-2"/>
  </process>
</definitions>`

	p := NewParser()
	proc, err := p.Parse([]byte(xml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	flow2 := proc.Flows["flow-2"]
	if flow2.Condition != "${approved}" {
		t.Errorf("expected condition ${approved}, got %s", flow2.Condition)
	}

	// Gateway should have condition populated from flow-level conditionExpression
	gw := proc.Elements["gw-1"]
	cond, ok := gw.Conditions["flow-2"]
	if !ok {
		t.Errorf("expected gateway to have condition for flow-2 from flow conditionExpression")
	}
	if cond != "${approved}" {
		t.Errorf("expected '${approved}', got '%s'", cond)
	}
}

func TestParser_Parse_ExclusiveGateway(t *testing.T) {
	xml := `<?xml version="1.0" encoding="UTF-8"?>
<definitions xmlns="http://www.omg.org/spec/BPMN/20100524/MODEL" targetNamespace="test">
  <process id="proc-2" name="Gateway Process">
    <startEvent id="start-1"/>
    <sequenceFlow id="flow-1" sourceRef="start-1" targetRef="gw-1"/>
    <exclusiveGateway id="gw-1" name="Decision" default="flow-3"/>
    <sequenceFlow id="flow-2" sourceRef="gw-1" targetRef="end-1" conditionExpression="${approved}"/>
    <sequenceFlow id="flow-3" sourceRef="gw-1" targetRef="end-2"/>
    <endEvent id="end-1"/>
    <endEvent id="end-2"/>
  </process>
</definitions>`

	p := NewParser()
	proc, err := p.Parse([]byte(xml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	gw := proc.Elements["gw-1"]
	if gw.Type != ElementTypeExclusiveGateway {
		t.Errorf("expected exclusiveGateway, got %s", gw.Type)
	}
	if gw.GatewayType != GatewayTypeExclusive {
		t.Errorf("expected gateway type exclusive, got %s", gw.GatewayType)
	}
	if gw.DefaultFlowID != "flow-3" {
		t.Errorf("expected default flow-3, got %s", gw.DefaultFlowID)
	}
	if len(gw.OutgoingFlows) != 2 {
		t.Errorf("expected 2 outgoing flows, got %d", len(gw.OutgoingFlows))
	}

	flow2 := proc.Flows["flow-2"]
	if flow2.Condition != "${approved}" {
		t.Errorf("expected condition ${approved}, got %s", flow2.Condition)
	}
}

func TestParser_Parse_UserTask(t *testing.T) {
	xml := `<?xml version="1.0" encoding="UTF-8"?>
<definitions xmlns="http://www.omg.org/spec/BPMN/20100524/MODEL" targetNamespace="test">
  <process id="proc-3" name="Task Process">
    <startEvent id="start-1"/>
    <sequenceFlow id="flow-1" sourceRef="start-1" targetRef="task-1"/>
    <userTask id="task-1" name="Review" assignee="user-1" candidateUsers="user-2,user-3" candidateGroups="managers"/>
    <sequenceFlow id="flow-2" sourceRef="task-1" targetRef="end-1"/>
    <endEvent id="end-1"/>
  </process>
</definitions>`

	p := NewParser()
	proc, err := p.Parse([]byte(xml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	task := proc.Elements["task-1"]
	if task.Type != ElementTypeUserTask {
		t.Errorf("expected userTask, got %s", task.Type)
	}
	if task.Assignee != "user-1" {
		t.Errorf("expected assignee user-1, got %s", task.Assignee)
	}
	if len(task.CandidateUsers) != 2 {
		t.Errorf("expected 2 candidate users, got %d", len(task.CandidateUsers))
	}
	if len(task.CandidateGroups) != 1 || task.CandidateGroups[0] != "managers" {
		t.Errorf("expected candidate group managers, got %v", task.CandidateGroups)
	}
}

func TestParser_Parse_TimerEvent(t *testing.T) {
	xml := `<?xml version="1.0" encoding="UTF-8"?>
<definitions xmlns="http://www.omg.org/spec/BPMN/20100524/MODEL" targetNamespace="test">
  <process id="proc-4" name="Timer Process">
    <startEvent id="start-1"/>
    <sequenceFlow id="flow-1" sourceRef="start-1" targetRef="timer-1"/>
    <intermediateCatchEvent id="timer-1">
      <timerEventDefinition>
        <timeDuration>PT1H</timeDuration>
      </timerEventDefinition>
    </intermediateCatchEvent>
    <sequenceFlow id="flow-2" sourceRef="timer-1" targetRef="end-1"/>
    <endEvent id="end-1"/>
  </process>
</definitions>`

	p := NewParser()
	proc, err := p.Parse([]byte(xml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	timer := proc.Elements["timer-1"]
	if timer.EventDefinition.Type != EventTypeTimer {
		t.Errorf("expected timer event type, got %s", timer.EventDefinition.Type)
	}
	if timer.EventDefinition.TimerType != TimerTypeDuration {
		t.Errorf("expected timer type duration, got %s", timer.EventDefinition.TimerType)
	}
	if timer.EventDefinition.TimerValue != "PT1H" {
		t.Errorf("expected timer value PT1H, got %s", timer.EventDefinition.TimerValue)
	}
}

func TestParser_Parse_SubProcess(t *testing.T) {
	xml := `<?xml version="1.0" encoding="UTF-8"?>
<definitions xmlns="http://www.omg.org/spec/BPMN/20100524/MODEL" targetNamespace="test">
  <process id="proc-sub" name="Sub Process">
    <startEvent id="start-1"/>
    <subProcess id="sp-1" name="My Sub">
      <startEvent id="sp-start"/>
      <userTask id="sp-task" assignee="user-1"/>
      <endEvent id="sp-end"/>
      <sequenceFlow id="sp-flow-1" sourceRef="sp-start" targetRef="sp-task"/>
      <sequenceFlow id="sp-flow-2" sourceRef="sp-task" targetRef="sp-end"/>
    </subProcess>
    <sequenceFlow id="flow-1" sourceRef="start-1" targetRef="sp-1"/>
    <sequenceFlow id="flow-2" sourceRef="sp-1" targetRef="end-1"/>
    <endEvent id="end-1"/>
  </process>
</definitions>`
	p := NewParser()
	proc, err := p.Parse([]byte(xml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	sp, ok := proc.Elements["sp-1"]
	if !ok {
		t.Fatal("expected sub-process element sp-1")
	}
	if sp.Type != ElementTypeSubProcess {
		t.Errorf("expected subProcess, got %s", sp.Type)
	}
	if sp.SubProcess == nil {
		t.Fatal("expected SubProcess to be non-nil")
	}
	if sp.SubProcessEnd == "" {
		t.Fatal("expected SubProcessEnd to be set")
	}
	// Flattened elements should have prefixed IDs
	if _, ok := proc.Elements["sp-1.sp-start"]; !ok {
		t.Error("expected flattened sp-1.sp-start element")
	}
	if _, ok := proc.Elements["sp-1.sp-task"]; !ok {
		t.Error("expected flattened sp-1.sp-task element")
	}
	if _, ok := proc.Elements["sp-1.sp-end"]; !ok {
		t.Error("expected flattened sp-1.sp-end element")
	}
	// Verify subprocess_exit_flows on internal end event
	spEnd := proc.Elements["sp-1.sp-end"]
	if spEnd.ExtensionData == nil || spEnd.ExtensionData["subprocess_exit_flows"] == "" {
		t.Error("expected subprocess_exit_flows on flattened end event")
	}
	// Synthetic entry flow should exist
	if _, ok := proc.Flows["sp-1_sp_entry"]; !ok {
		t.Error("expected synthetic entry flow sp-1_sp_entry")
	}
}

func TestParser_Parse_ErrorEndEvent(t *testing.T) {
	xml := `<?xml version="1.0" encoding="UTF-8"?>
<definitions xmlns="http://www.omg.org/spec/BPMN/20100524/MODEL" targetNamespace="test">
  <process id="proc-err" name="Error Process">
    <startEvent id="start-1"/>
    <sequenceFlow id="flow-1" sourceRef="start-1" targetRef="err-end-1"/>
    <endEvent id="err-end-1">
      <errorEventDefinition errorRef="ERR-001"/>
    </endEvent>
    <sequenceFlow id="flow-2" sourceRef="err-end-1" targetRef="end-1"/>
    <endEvent id="end-1"/>
  </process>
</definitions>`
	p := NewParser()
	proc, err := p.Parse([]byte(xml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	elem := proc.Elements["err-end-1"]
	if elem.Type != ElementTypeErrorEnd {
		t.Errorf("expected errorEnd, got %s", elem.Type)
	}
	if elem.EventDefinition.Type != EventTypeError {
		t.Errorf("expected event type error, got %s", elem.EventDefinition.Type)
	}
	if elem.EventDefinition.ErrorCode != "ERR-001" {
		t.Errorf("expected errorCode ERR-001, got %s", elem.EventDefinition.ErrorCode)
	}
}

func TestParser_Parse_ErrorBoundaryEvent(t *testing.T) {
	xml := `<?xml version="1.0" encoding="UTF-8"?>
<definitions xmlns="http://www.omg.org/spec/BPMN/20100524/MODEL" targetNamespace="test">
  <process id="proc-err-bound" name="Error Boundary">
    <startEvent id="start-1"/>
    <subProcess id="sp-1" name="Sub">
      <startEvent id="sp-start"/>
      <endEvent id="sp-end"/>
      <sequenceFlow id="sp-flow-1" sourceRef="sp-start" targetRef="sp-end"/>
    </subProcess>
    <boundaryEvent id="err-catch-1" attachedToRef="sp-1">
      <errorEventDefinition errorRef="ERR-001"/>
    </boundaryEvent>
    <sequenceFlow id="flow-catch" sourceRef="err-catch-1" targetRef="end-catch"/>
    <endEvent id="end-catch"/>
    <sequenceFlow id="flow-1" sourceRef="start-1" targetRef="sp-1"/>
  </process>
</definitions>`
	p := NewParser()
	proc, err := p.Parse([]byte(xml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	elem := proc.Elements["err-catch-1"]
	if elem.Type != ElementTypeErrorCatch {
		t.Errorf("expected errorCatch, got %s", elem.Type)
	}
	if elem.AttachedToRef != "sp-1" {
		t.Errorf("expected AttachedToRef=sp-1, got %s", elem.AttachedToRef)
	}
	if elem.EventDefinition.ErrorCode != "ERR-001" {
		t.Errorf("expected ErrorCode=ERR-001, got %s", elem.EventDefinition.ErrorCode)
	}
}

func TestParser_Parse_SignalEvent(t *testing.T) {
	xml := `<?xml version="1.0" encoding="UTF-8"?>
<definitions xmlns="http://www.omg.org/spec/BPMN/20100524/MODEL" targetNamespace="test">
  <process id="proc-sig" name="Signal Process">
    <startEvent id="start-1"/>
    <sequenceFlow id="flow-1" sourceRef="start-1" targetRef="sig-throw-1"/>
    <intermediateThrowEvent id="sig-throw-1">
      <signalEventDefinition signalRef="sig-1"/>
    </intermediateThrowEvent>
    <sequenceFlow id="flow-2" sourceRef="sig-throw-1" targetRef="sig-catch-1"/>
    <intermediateCatchEvent id="sig-catch-1">
      <signalEventDefinition signalRef="sig-1"/>
    </intermediateCatchEvent>
    <sequenceFlow id="flow-3" sourceRef="sig-catch-1" targetRef="end-1"/>
    <endEvent id="end-1"/>
  </process>
</definitions>`
	p := NewParser()
	proc, err := p.Parse([]byte(xml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	throw := proc.Elements["sig-throw-1"]
	if throw.Type != ElementTypeSignalThrow {
		t.Errorf("expected signalThrow, got %s", throw.Type)
	}
	if throw.EventDefinition.SignalRef != "sig-1" {
		t.Errorf("expected SignalRef=sig-1, got %s", throw.EventDefinition.SignalRef)
	}
	catch := proc.Elements["sig-catch-1"]
	if catch.Type != ElementTypeSignalCatch {
		t.Errorf("expected signalCatch, got %s", catch.Type)
	}
	if catch.EventDefinition.SignalRef != "sig-1" {
		t.Errorf("expected SignalRef=sig-1, got %s", catch.EventDefinition.SignalRef)
	}
}

func TestParser_Parse_BoundaryTimerWithCancelActivity(t *testing.T) {
	xml := `<?xml version="1.0" encoding="UTF-8"?>
<definitions xmlns="http://www.omg.org/spec/BPMN/20100524/MODEL" targetNamespace="test">
  <process id="proc-bound" name="Boundary">
    <startEvent id="start-1"/>
    <userTask id="task-1" assignee="user-1"/>
    <boundaryEvent id="timer-1" attachedToRef="task-1" cancelActivity="false">
      <timerEventDefinition>
        <timeDuration>PT1H</timeDuration>
      </timerEventDefinition>
    </boundaryEvent>
    <sequenceFlow id="flow-1" sourceRef="start-1" targetRef="task-1"/>
    <sequenceFlow id="flow-bound" sourceRef="timer-1" targetRef="end-bound"/>
    <endEvent id="end-bound"/>
  </process>
</definitions>`
	p := NewParser()
	proc, err := p.Parse([]byte(xml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	timer := proc.Elements["timer-1"]
	if timer.Type != ElementTypeTimerEvent {
		t.Errorf("expected timerEvent, got %s", timer.Type)
	}
	if timer.AttachedToRef != "task-1" {
		t.Errorf("expected AttachedToRef=task-1, got %s", timer.AttachedToRef)
	}
	if timer.CancelActivity != false {
		t.Errorf("expected CancelActivity=false for cancelActivity=\"false\", got %v", timer.CancelActivity)
	}
	if timer.EventDefinition.TimerType != TimerTypeDuration {
		t.Errorf("expected TimerType=Duration, got %s", timer.EventDefinition.TimerType)
	}
	if timer.EventDefinition.TimerValue != "PT1H" {
		t.Errorf("expected TimerValue=PT1H, got %s", timer.EventDefinition.TimerValue)
	}
}

func TestParser_Parse_IntermediateCatchToTimer(t *testing.T) {
	xml := `<?xml version="1.0" encoding="UTF-8"?>
<definitions xmlns="http://www.omg.org/spec/BPMN/20100524/MODEL" targetNamespace="test">
  <process id="proc-timer-catch" name="Timer Catch">
    <startEvent id="start-1"/>
    <sequenceFlow id="flow-1" sourceRef="start-1" targetRef="timer-1"/>
    <intermediateCatchEvent id="timer-1">
      <timerEventDefinition>
        <timeDuration>PT30M</timeDuration>
      </timerEventDefinition>
    </intermediateCatchEvent>
    <sequenceFlow id="flow-2" sourceRef="timer-1" targetRef="end-1"/>
    <endEvent id="end-1"/>
  </process>
</definitions>`
	p := NewParser()
	proc, err := p.Parse([]byte(xml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	elem := proc.Elements["timer-1"]
	if elem.Type != ElementTypeTimerEvent {
		t.Errorf("expected timerEvent for intermediateCatchEvent with timerEventDefinition, got %s", elem.Type)
	}
}

func TestParser_Parse_IntermediateCatchToMessage(t *testing.T) {
	xml := `<?xml version="1.0" encoding="UTF-8"?>
<definitions xmlns="http://www.omg.org/spec/BPMN/20100524/MODEL" targetNamespace="test">
  <process id="proc-msg-catch" name="Message Catch">
    <startEvent id="start-1"/>
    <sequenceFlow id="flow-1" sourceRef="start-1" targetRef="msg-1"/>
    <intermediateCatchEvent id="msg-1">
      <messageEventDefinition messageRef="msg-1"/>
    </intermediateCatchEvent>
    <sequenceFlow id="flow-2" sourceRef="msg-1" targetRef="end-1"/>
    <endEvent id="end-1"/>
  </process>
</definitions>`
	p := NewParser()
	proc, err := p.Parse([]byte(xml))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	elem := proc.Elements["msg-1"]
	if elem.Type != ElementTypeMessageCatch {
		t.Errorf("expected messageCatch for intermediateCatchEvent with messageEventDefinition, got %s", elem.Type)
	}
}

func FuzzParseBPMN(f *testing.F) {
	f.Add([]byte(`<?xml version="1.0"?><definitions xmlns="http://www.omg.org/spec/BPMN/20100524/MODEL"><process id="p1"><startEvent id="s1"/></process></definitions>`))
	f.Add([]byte(`not xml at all`))
	f.Add([]byte(``))
	f.Add([]byte(`<?xml version="1.0"?><definitions/>`))

	parser := NewParser()
	f.Fuzz(func(t *testing.T, data []byte) {
		// Should never panic
		_, _ = parser.Parse(data)
	})
}
