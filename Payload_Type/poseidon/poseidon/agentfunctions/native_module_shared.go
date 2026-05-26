package agentfunctions

import (
	"fmt"
	"strings"

	agentstructs "github.com/MythicMeta/MythicContainer/agent_structs"
	"github.com/MythicMeta/MythicContainer/logging"
	"github.com/MythicMeta/MythicContainer/mythicrpc"
)

const nativeModuleComment = "Loaded for native_import/native_call/native_unload"

func formatNativeModuleSelection(filename, agentFileID string) string {
	return fmt.Sprintf("%s (%s)", filename, agentFileID)
}

func parseNativeModuleFileID(selected string) string {
	if start := strings.LastIndex(selected, " ("); start >= 0 && strings.HasSuffix(selected, ")") {
		fileID := selected[start+2 : len(selected)-1]
		if start > 0 && isUUIDish(fileID) {
			return fileID
		}
	}
	return selected
}

func isUUIDish(value string) bool {
	if len(value) != 36 {
		return false
	}
	for i, r := range value {
		switch i {
		case 8, 13, 18, 23:
			if r != '-' {
				return false
			}
		default:
			if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
				return false
			}
		}
	}
	return true
}

func hasNativeModuleComment(comment string) bool {
	return strings.Contains(comment, nativeModuleComment)
}

func getNativeModuleFiles(input agentstructs.PTRPCDynamicQueryFunctionMessage) []string {
	fileResp, err := mythicrpc.SendMythicRPCFileSearch(mythicrpc.MythicRPCFileSearchMessage{
		LimitByCallback:     true,
		CallbackID:          input.Callback,
		IsPayload:           false,
		IsDownloadFromAgent: false,
	})
	if err != nil {
		logging.LogError(err, "Failed to search for native module files in callback")
		return []string{}
	}
	if !fileResp.Success {
		logging.LogError(nil, "Failed to search for native module files in callback", "mythic error", fileResp.Error)
		return []string{}
	}

	options := make([]string, 0, len(fileResp.Files))
	for _, file := range fileResp.Files {
		if !hasNativeModuleComment(file.Comment) {
			continue
		}
		options = append(options, formatNativeModuleSelection(file.Filename, file.AgentFileID))
	}
	return options
}

func resolveNativeModuleFile(selected string, callbackID int) (*mythicrpc.FileData, string) {
	fileID := parseNativeModuleFileID(selected)
	search, err := mythicrpc.SendMythicRPCFileSearch(mythicrpc.MythicRPCFileSearchMessage{
		AgentFileID:         fileID,
		LimitByCallback:     true,
		CallbackID:          callbackID,
		IsPayload:           false,
		IsDownloadFromAgent: false,
	})
	if err != nil {
		return nil, "Error trying to search for files: " + err.Error()
	}
	if !search.Success {
		return nil, search.Error
	}
	if len(search.Files) == 0 {
		return nil, "Failed to find specified file"
	}
	file := search.Files[0]
	if !hasNativeModuleComment(file.Comment) {
		return nil, "Selected file is not a loaded native module"
	}
	return &file, ""
}
