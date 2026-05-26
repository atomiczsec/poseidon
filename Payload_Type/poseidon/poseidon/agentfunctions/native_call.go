package agentfunctions

import (
	"fmt"
	"strings"

	agentstructs "github.com/MythicMeta/MythicContainer/agent_structs"
	"github.com/MythicMeta/MythicContainer/logging"
	"github.com/MythicMeta/MythicContainer/mythicrpc"
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
				Name:                 "filename",
				ModalDisplayName:     "Module Loaded via native_import",
				ParameterType:        agentstructs.COMMAND_PARAMETER_TYPE_CHOOSE_ONE,
				Description:          "The filename of the imported .so or .dylib.",
				DynamicQueryFunction: getCallbackFiles,
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
			if filename, err := taskData.Args.GetStringArg("filename"); err != nil {
				logging.LogError(err, "Failed to get filename")
				response.Success = false
				response.Error = err.Error()
				return response
			} else if search, err := mythicrpc.SendMythicRPCFileSearch(mythicrpc.MythicRPCFileSearchMessage{
				Filename:            filename,
				LimitByCallback:     true,
				CallbackID:          taskData.Callback.ID,
				MaxResults:          1,
				IsPayload:           false,
				IsDownloadFromAgent: false,
			}); err != nil {
				response.Success = false
				response.Error = "Error trying to search for files: " + err.Error()
				return response
			} else if !search.Success {
				response.Success = false
				response.Error = search.Error
				return response
			} else if len(search.Files) == 0 {
				response.Success = false
				response.Error = "Failed to find specified file"
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
				taskData.Args.RemoveArg("filename")
				taskData.Args.AddArg(agentstructs.CommandParameter{
					Name:          "file_id",
					ParameterType: agentstructs.COMMAND_PARAMETER_TYPE_STRING,
					DefaultValue:  search.Files[0].AgentFileID,
				})
				displayString := fmt.Sprintf("function %s of %s", functionName, search.Files[0].Filename)
				if len(callArgs) > 0 {
					displayString = fmt.Sprintf("%s with args %s", displayString, strings.Join(callArgs, " "))
				}
				response.DisplayParams = &displayString
				return response
			}
		},
	})
}
