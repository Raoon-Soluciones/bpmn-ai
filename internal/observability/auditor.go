package observability

type Auditor struct {
	dispatcher *Dispatcher
	writer     *FileAuditWriter
}

func NewAuditor(dispatcher *Dispatcher, writer *FileAuditWriter) *Auditor {
	a := &Auditor{
		dispatcher: dispatcher,
		writer:     writer,
	}

	eventTypes := []string{
		EventProcessStarted,
		EventProcessCompleted,
		EventProcessTerminated,
		EventProcessError,
		EventElementExecuted,
		EventElementError,
		EventTaskClaimed,
		EventTaskCompleted,
		EventJobQueued,
		EventJobCompleted,
		EventJobFailed,
		EventJobDead,
	}

	for _, et := range eventTypes {
		dispatcher.Register(et, writer.HandleEvent)
	}

	return a
}

func (a *Auditor) Dispatcher() *Dispatcher {
	return a.dispatcher
}
