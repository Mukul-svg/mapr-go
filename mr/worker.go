package mr

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"log"
	"net/rpc"
	"os"
	"sort"
	"time"
)

// KeyValue is the fundamental data unit passed between Map and Reduce.
type KeyValue struct {
	Key   string
	Value string
}

// ihash maps a key to a reduce bucket: ihash(key) % NReduce.
func ihash(key string) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	return int(h.Sum32() & 0x7fffffff)
}

// coordSockName is set once at startup so helpers can reach the coordinator.
var coordSockName string

// Worker is the main worker loop. It continuously asks the coordinator for
// tasks and executes them until the coordinator signals "exit".
//
//   - sockname: Unix socket path of the coordinator
//   - mapf: user-provided Map function
//   - reducef: user-provided Reduce function
func Worker(sockname string, mapf func(string, string) []KeyValue,
	reducef func(string, []string) string) {

	coordSockName = sockname

	for {
		task := getTask()

		switch task.TaskType {
		case "map":
			doMap(task, mapf)
		case "reduce":
			doReduce(task, reducef)
		case "wait":
			time.Sleep(time.Second)
		case "exit":
			return
		}
	}
}

// doMap executes a single map task:
//  1. Read the input file
//  2. Call mapf to get key/value pairs
//  3. Hash-partition them into NReduce intermediate files (mr-<mapID>-<reduceID>)
//  4. Atomically rename to avoid partial writes
func doMap(task GetTaskReply, mapf func(string, string) []KeyValue) {
	buckets := make([][]KeyValue, task.NReduce)

	file, err := os.Open(task.File)
	if err != nil {
		log.Fatalf("cannot open %v", task.File)
	}
	content, err := io.ReadAll(file)
	if err != nil {
		log.Fatalf("cannot read %v", task.File)
	}
	file.Close()

	kva := mapf(task.File, string(content))
	for _, kv := range kva {
		bucketID := ihash(kv.Key) % task.NReduce
		buckets[bucketID] = append(buckets[bucketID], kv)
	}

	for bucketID, kvs := range buckets {
		finalName := fmt.Sprintf("mr-%d-%d", task.TaskID, bucketID)

		// Write to a temp file, then atomically rename (crash safety).
		tmpFile, err := os.CreateTemp(".", "mr-tmp-*")
		if err != nil {
			log.Println("CreateTemp:", err)
			continue
		}
		enc := json.NewEncoder(tmpFile)
		for _, kv := range kvs {
			enc.Encode(&kv)
		}
		tmpFile.Close()
		os.Rename(tmpFile.Name(), finalName)
	}

	reportTaskDone(task)
}

// doReduce executes a single reduce task:
//  1. Collect all intermediate files for this reduce partition
//  2. Sort by key
//  3. Call reducef for each distinct key
//  4. Write output to mr-out-<reduceID> atomically
func doReduce(task GetTaskReply, reducef func(string, []string) string) {
	intermediate := []KeyValue{}

	for m := 0; m < task.NMap; m++ {
		fileName := fmt.Sprintf("mr-%d-%d", m, task.TaskID)
		file, err := os.Open(fileName)
		if err != nil {
			continue // not all map tasks write to every bucket
		}
		dec := json.NewDecoder(file)
		for {
			var kv KeyValue
			if err := dec.Decode(&kv); err != nil {
				break
			}
			intermediate = append(intermediate, kv)
		}
		file.Close()
	}

	sort.Slice(intermediate, func(i, j int) bool {
		return intermediate[i].Key < intermediate[j].Key
	})

	outName := fmt.Sprintf("mr-out-%d", task.TaskID)
	tmpFile, _ := os.CreateTemp(".", "mr-out-tmp-*")

	i := 0
	for i < len(intermediate) {
		j := i + 1
		for j < len(intermediate) && intermediate[j].Key == intermediate[i].Key {
			j++
		}
		values := []string{}
		for k := i; k < j; k++ {
			values = append(values, intermediate[k].Value)
		}
		output := reducef(intermediate[i].Key, values)
		fmt.Fprintf(tmpFile, "%v %v\n", intermediate[i].Key, output)
		i = j
	}

	tmpFile.Close()
	os.Rename(tmpFile.Name(), outName)

	reportTaskDone(task)
}

// reportTaskDone tells the coordinator that a task has finished.
func reportTaskDone(task GetTaskReply) bool {
	args := ReportTaskDoneArgs{
		TaskType: task.TaskType,
		TaskID:   task.TaskID,
		File:     task.File,
	}
	reply := ReportTaskDoneReply{}
	return call("Coordinator.ReportTaskDone", &args, &reply)
}

// getTask asks the coordinator for the next task.
func getTask() GetTaskReply {
	args := GetTaskArgs{WorkerID: os.Getpid()}
	reply := GetTaskReply{}
	ok := call("Coordinator.GetTask", &args, &reply)
	if ok {
		return reply
	}
	return GetTaskReply{}
}

// call makes an RPC call to the coordinator over the Unix socket.
func call(rpcname string, args interface{}, reply interface{}) bool {
	c, err := rpc.DialHTTP("unix", coordSockName)
	if err != nil {
		log.Fatal("dialing:", err)
	}
	defer c.Close()

	if err := c.Call(rpcname, args, reply); err == nil {
		return true
	}
	log.Printf("[worker %d] call failed: %v", os.Getpid(), err)
	return false
}
