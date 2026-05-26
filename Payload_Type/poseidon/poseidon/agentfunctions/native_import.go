package agentfunctions

import (
	"fmt"
	"strings"

	agentstructs "github.com/MythicMeta/MythicContainer/agent_structs"
	"github.com/MythicMeta/MythicContainer/logging"
	"github.com/MythicMeta/MythicContainer/mythicrpc"
)

const nativeImportClearCommentCompletion = "clear_native_import_comment_on_failure"

func init() {
	agentstructs.AllPayloadData.Get("poseidon").AddCommand(agentstructs.Command{
		Name:                "native_import",
		HelpString:          "native_import",
		Description:         "Upload a Linux .so or macOS .dylib and keep it loaded for later native_call tasks. The module must export C functions as char* func(int argc, char** argv).",
		Version:             1,
		MitreAttackMappings: []string{"T1105", "T1620"},
		Author:              "@its_a_feature_",
		CommandParameters: []agentstructs.CommandParameter{
			{
				Name:             "file_id",
				ModalDisplayName: "Native Module to Load",
				ParameterType:    agentstructs.COMMAND_PARAMETER_TYPE_FILE,
				Description:      "Select the .so or .dylib to load into the agent process.",
				ParameterGroupInformation: []agentstructs.ParameterGroupInfo{
					{
						ParameterIsRequired: true,
						UIModalPosition:     1,
					},
				},
			},
		},
		CommandAttributes: agentstructs.CommandAttribute{
			SupportedOS:        []string{agentstructs.SUPPORTED_OS_MACOS, agentstructs.SUPPORTED_OS_LINUX},
			CommandIsSuggested: true,
		},
		TaskCompletionFunctions: map[string]agentstructs.PTTaskCompletionFunction{
			nativeImportClearCommentCompletion: clearNativeImportCommentOnFailure,
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
			if fileID, err := taskData.Args.GetStringArg("file_id"); err != nil {
				logging.LogError(err, "Failed to get file_id")
				response.Success = false
				response.Error = err.Error()
				return response
			} else if search, err := mythicrpc.SendMythicRPCFileSearch(mythicrpc.MythicRPCFileSearchMessage{
				AgentFileID: fileID,
			}); err != nil {
				response.Success = false
				response.Error = err.Error()
				return response
			} else if !search.Success {
				response.Success = false
				response.Error = search.Error
				return response
			} else if len(search.Files) == 0 {
				response.Success = false
				response.Error = "Failed to find specified file"
				return response
			} else if _, err := mythicrpc.SendMythicRPCFileUpdate(mythicrpc.MythicRPCFileUpdateMessage{
				AgentFileID: fileID,
				Comment:     nativeModuleComment,
			}); err != nil {
				response.Success = false
				response.Error = err.Error()
				return response
			} else {
				displayString := fmt.Sprintf("module %s", search.Files[0].Filename)
				response.DisplayParams = &displayString
				completionName := nativeImportClearCommentCompletion
				response.CompletionFunctionName = &completionName
				return response
			}
		},
	})
}

func clearNativeImportCommentOnFailure(taskData *agentstructs.PTTaskMessageAllData, _ *agentstructs.PTTaskMessageAllData, _ *agentstructs.SubtaskGroupName) agentstructs.PTTaskCompletionFunctionMessageResponse {
	response := agentstructs.PTTaskCompletionFunctionMessageResponse{
		Success: true,
		TaskID:  taskData.Task.ID,
	}
	if !nativeImportTaskFailed(taskData.Task.Status) {
		return response
	}

	fileID, err := taskData.Args.GetStringArg("file_id")
	if err != nil {
		logging.LogError(err, "Failed to get file_id for native_import cleanup")
		response.Success = false
		response.Error = err.Error()
		return response
	}
	search, err := mythicrpc.SendMythicRPCFileSearch(mythicrpc.MythicRPCFileSearchMessage{
		AgentFileID: fileID,
	})
	if err != nil {
		response.Success = false
		response.Error = err.Error()
		return response
	}
	if !search.Success {
		response.Success = false
		response.Error = search.Error
		return response
	}
	if len(search.Files) == 0 || !hasNativeModuleComment(search.Files[0].Comment) {
		return response
	}

	comment := strings.TrimSpace(strings.Replace(search.Files[0].Comment, nativeModuleComment, "", 1))
	update, err := mythicrpc.SendMythicRPCFileUpdate(mythicrpc.MythicRPCFileUpdateMessage{
		AgentFileID: fileID,
		Comment:     comment,
	})
	if err != nil {
		response.Success = false
		response.Error = err.Error()
		return response
	}
	if !update.Success {
		response.Success = false
		response.Error = update.Error
		return response
	}
	return response
}

func nativeImportTaskFailed(status string) bool {
	status = strings.ToLower(strings.TrimSpace(status))
	return status == "error" || strings.HasPrefix(status, "error:")
}
