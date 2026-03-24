package mr

import (
	"log"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"sync"
	"time"
)

// TaskStatus tracks the lifecycle of each map/reduce task.
type TaskStatus int

const (
	Idle       TaskStatus = iota
	Inprogress            // assigned to a worker
	Completed
)

// TaskMetadata holds the state of a single map or reduce task.
type TaskMetadata struct {
	TaskID    int
	Type      string
	File      string
	Status    TaskStatus
	StartTime time.Time
}

// Coordinator manages the overall MapReduce job:
//   - Assigns map tasks (one per input file) to workers
//   - Once all maps are done, assigns reduce tasks (nReduce total)
//   - Re-assigns tasks that take longer than 10 seconds (fault tolerance)
type Coordinator struct {
	mu sync.RWMutex

	mapTasks    map[int]*TaskMetadata
	reduceTasks map[int]*TaskMetadata

	nReduce         int
	mapTasksDone    int
	reduceTasksDone int
	isDone          bool
}

// GetTask is called by workers to get a new task.
// It hands out map tasks first, then reduce tasks once all maps are done.
func (c *Coordinator) GetTask(args *GetTaskArgs, reply *GetTaskReply) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	nMap := len(c.mapTasks)

	// Hand out any idle map task.
	for _, mapTask := range c.mapTasks {
		if mapTask.Status == Idle {
			*reply = GetTaskReply{
				TaskType: "map",
				TaskID:   mapTask.TaskID,
				File:     mapTask.File,
				NReduce:  c.nReduce,
				NMap:     nMap,
			}
			mapTask.Status = Inprogress
			mapTask.StartTime = time.Now()
			go c.TimeoutMonitor(mapTask)
			return nil
		}
	}

	// All map tasks assigned but not all finished yet — tell worker to wait.
	if nMap != c.mapTasksDone {
		*reply = GetTaskReply{TaskType: "wait"}
		return nil
	}

	// Hand out any idle reduce task.
	for _, reduceTask := range c.reduceTasks {
		if reduceTask.Status == Idle {
			*reply = GetTaskReply{
				TaskType: "reduce",
				TaskID:   reduceTask.TaskID,
				File:     reduceTask.File,
				NReduce:  c.nReduce,
				NMap:     nMap,
			}
			reduceTask.Status = Inprogress
			reduceTask.StartTime = time.Now()
			go c.TimeoutMonitor(reduceTask)
			return nil
		}
	}

	// All done — tell workers to exit.
	if c.nReduce == c.reduceTasksDone {
		*reply = GetTaskReply{TaskType: "exit"}
		c.isDone = true
		return nil
	}

	// All reduce tasks assigned but not finished yet.
	*reply = GetTaskReply{TaskType: "wait"}
	return nil
}

// ReportTaskDone is called by a worker after completing a task.
// When all map tasks finish, it creates the reduce task list.
func (c *Coordinator) ReportTaskDone(args *ReportTaskDoneArgs, reply *ReportTaskDoneReply) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if args.TaskType == "map" && c.mapTasks[args.TaskID].Status != Completed {
		c.mapTasks[args.TaskID].Status = Completed
		c.mapTasksDone++

		// Once all maps are done, initialize the reduce tasks.
		if c.mapTasksDone == len(c.mapTasks) {
			for id := range c.nReduce {
				c.reduceTasks[id] = &TaskMetadata{
					TaskID: id,
					Type:   "reduce",
					Status: Idle,
				}
			}
		}

		*reply = ReportTaskDoneReply{Ok: true}
	}

	if args.TaskType == "reduce" && c.reduceTasks[args.TaskID].Status != Completed {
		c.reduceTasks[args.TaskID].Status = Completed
		c.reduceTasksDone++
		*reply = ReportTaskDoneReply{Ok: true}
	}

	return nil
}

// TimeoutMonitor re-queues a task if the assigned worker takes more than 10 seconds.
// This handles worker crashes and stragglers.
func (c *Coordinator) TimeoutMonitor(task *TaskMetadata) {
	time.Sleep(10 * time.Second)
	c.mu.Lock()
	defer c.mu.Unlock()
	if task.Status == Inprogress {
		task.Status = Idle
	}
}

// server starts an RPC server on the given Unix socket path.
func (c *Coordinator) server(sockname string) {
	rpc.Register(c)
	rpc.HandleHTTP()
	os.Remove(sockname)
	l, e := net.Listen("unix", sockname)
	if e != nil {
		log.Fatalf("listen error %s: %v", sockname, e)
	}
	go http.Serve(l, nil)
}

// Done returns true once all reduce tasks have completed.
// Called periodically by the coordinator's main loop.
func (c *Coordinator) Done() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.isDone
}

// MakeCoordinator creates and starts the coordinator.
//   - sockname: Unix socket path for worker communication
//   - files: list of input files (one map task per file)
//   - nReduce: number of reduce partitions
func MakeCoordinator(sockname string, files []string, nReduce int) *Coordinator {
	c := Coordinator{
		mapTasks:    make(map[int]*TaskMetadata),
		reduceTasks: make(map[int]*TaskMetadata),
		nReduce:     nReduce,
	}

	for id, file := range files {
		c.mapTasks[id] = &TaskMetadata{
			TaskID: id,
			Type:   "map",
			File:   file,
			Status: Idle,
		}
	}

	c.server(sockname)
	return &c
}
