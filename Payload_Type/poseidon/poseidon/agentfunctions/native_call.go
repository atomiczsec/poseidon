package agentfunctions

import (
	"fmt"
	"strings"

	agentstructs "github.com/MythicMeta/MythicContainer/agent_structs"
	"github.com/MythicMeta/MythicContainer/logging"
)

func init() {
	agentstructs.AllPayloadData.Get("poseidon").AddCommand(agentstructs.Command{
		Name:                "native_call",
		HelpString:          "native_call",
		Description:         "Run an exported C function from a module previously loaded with native_import.",
		Version:             1,
		MitreAttackMappings: []string{"T1106", "T1620"},
		Author:              "@its_a_feature_",
		CommandParameters: []agentstructs.CommandParameter{
			{
				Name:                 "file_id",
				ModalDisplayName:     "Module Loaded via native_import",
				ParameterType:        agentstructs.COMMAND_PARAMETER_TYPE_CHOOSE_ONE,
				Description:          "The native module previously imported with native_import, shown as filename (file_id).",
				DynamicQueryFunction: getNativeModuleFiles,
				ParameterGroupInformation: []agentstructs.ParameterGroupInfo{
					{
						ParameterIsRequired: true,
						UIModalPosition:     1,
					},
				},
			},
			{
				Name:          "function_name",
				ParameterType: agentstructs.COMMAND_PARAMETER_TYPE_STRING,
				Description:   "The exported function to execute. The function must match char* func(int argc, char** argv).",
				ParameterGroupInformation: []agentstructs.ParameterGroupInfo{
					{
						ParameterIsRequired: true,
						UIModalPosition:     2,
					},
				},
			},
			{
				Name:             "args",
				ModalDisplayName: "Arguments",
				ParameterType:    agentstructs.COMMAND_PARAMETER_TYPE_ARRAY,
				Description:      "String arguments to pass to the exported function.",
				DefaultValue:     []string{},
				ParameterGroupInformation: []agentstructs.ParameterGroupInfo{
					{
						ParameterIsRequired: false,
						UIModalPosition:     3,
					},
				},
			},
		},
		CommandAttributes: agentstructs.CommandAttribute{
			SupportedOS:        []string{agentstructs.SUPPORTED_OS_MACOS, agentstructs.SUPPORTED_OS_LINUX},
			CommandIsSuggested: true,
		},
		TaskFunctionParseArgString: func(args *agentstructs.PTTaskMessageArgsData, input string) error {
			return args.LoadArgsFromJSONString(input)
		},
		TaskFunctionParseArgDictionary: func(args *agentstructs.PTTaskMessageArgsData, input map[string]interface{}) error {
			return args.LoadArgsFromDictionary(input)
		},
		TaskFunctionCreateTasking: func(taskData *agentstructs.PTTaskMessageAllData) agentstructs.PTTaskCreateTaskingMessageResponse {
			response := agentstructs.PTTaskCreateTaskingMessageResponse{
				Success: true,
				TaskID:  taskData.Task.ID,
			}
			if fileSelection, err := taskData.Args.GetStringArg("file_id"); err != nil {
				logging.LogError(err, "Failed to get file_id")
				response.Success = false
				response.Error = err.Error()
				return response
			} else if file, errMsg := resolveNativeModuleFile(fileSelection, taskData.Callback.ID); errMsg != "" {
				response.Success = false
				response.Error = errMsg
				return response
			} else if functionName, err := taskData.Args.GetStringArg("function_name"); err != nil {
				response.Success = false
				response.Error = err.Error()
				return response
			} else if callArgs, err := taskData.Args.GetArrayArg("args"); err != nil {
				response.Success = false
				response.Error = err.Error()
				return response
			} else {
				taskData.Args.RemoveArg("file_id")
				taskData.Args.AddArg(agentstructs.CommandParameter{
					Name:          "file_id",
					ParameterType: agentstructs.COMMAND_PARAMETER_TYPE_STRING,
					DefaultValue:  file.AgentFileID,
				})
				displayString := fmt.Sprintf("function %s of %s", functionName, file.Filename)
				if len(callArgs) > 0 {
					displayString = fmt.Sprintf("%s with args %s", displayString, strings.Join(callArgs, " "))
				}
				response.DisplayParams = &displayString
				return response
			}
		},
	})
}
