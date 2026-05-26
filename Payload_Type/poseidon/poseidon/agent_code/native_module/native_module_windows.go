//go:build windows && (native_import || native_call || native_unload || native_module || debug)

package native_module

import (
	"github.com/MythicAgents/poseidon/Payload_Type/poseidon/agent_code/pkg/tasks/taskRegistrar"
	"github.com/MythicAgents/poseidon/Payload_Type/poseidon/agent_code/pkg/utils/structs"
)

func init() {
	taskRegistrar.Register("native_import", runUnsupported)
	taskRegistrar.Register("native_call", runUnsupported)
	taskRegistrar.Register("native_unload", runUnsupported)
}

func runUnsupported(task structs.Task) {
	msg := task.NewResponse()
	msg.SetError("Native modules are not supported on Windows")
	task.Job.SendResponses <- msg
}
