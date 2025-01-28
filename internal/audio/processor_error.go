package audio

// setLoadError changes the processor status to an error state during file loading.
func (p *Processor) setLoadError(msg string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	logDebug("Load error: %s", msg)
	p.status = ProcessingStatus{
		State:   StateIdle,
		Message: msg,
	}
}

// setError changes the processor status to an error state at any stage of processing or analysis.
func (p *Processor) setError(msg string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	logDebug("Error: %s", msg)
	p.status = ProcessingStatus{
		State:   StateIdle,
		Message: msg,
	}
}

// setStatus updates the processor status with a general message and a new state.
func (p *Processor) setStatus(state ProcessingState, msg string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	logDebug("Status update: [%v] %s", state, msg)
	p.status = ProcessingStatus{
		State:   state,
		Message: msg,
	}
}

// updateProgressWithMessage updates processor status, including a progress float, for example during file reading.
func (p *Processor) updateProgressWithMessage(state ProcessingState, msg string, progress float64) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if progress < 0 {
		progress = 0
	}
	if progress > 1 {
		progress = 1
	}

	p.status = ProcessingStatus{
		State:     state,
		Message:   msg,
		Progress:  progress,
		CanCancel: true,
	}
}
