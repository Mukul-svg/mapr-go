package mr

// RPC definitions for Map/Reduce coordinator-worker communication.
// All names must be capitalized for RPC to work.

// GetTask: worker requests a task from the coordinator.
type GetTaskArgs struct {
	WorkerID int
}

type GetTaskReply struct {
	TaskType string // "map", "reduce", "wait", or "exit"
	TaskID   int
	File     string
	NReduce  int
	NMap     int
}

// ReportTaskDone: worker reports a completed task to the coordinator.
type ReportTaskDoneArgs struct {
	TaskType string
	TaskID   int
	File     string
}

type ReportTaskDoneReply struct {
	Ok bool
}
