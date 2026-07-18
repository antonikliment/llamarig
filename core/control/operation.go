package control

import (
	"cmp"

	"llamarig/core/runtime"
)

type OperationResult struct {
	Action, Target, Status, Message string
	DurationMS                      int64
}

func operationFromRuntimeResult(action string, target string, result runtime.CommandResult, err error) OperationResult {
	if result.Action != "" {
		action = result.Action
	}
	if err != nil {
		return OperationResult{Action: action, Target: target, Status: "failed", Message: err.Error(), DurationMS: result.DurationMS}
	}
	if result.ExitCode != 0 {
		message := cmp.Or(result.Stderr, result.Stdout, "command failed")
		return OperationResult{Action: action, Target: target, Status: "failed", Message: message, DurationMS: result.DurationMS}
	}
	if result.Stdout != "" {
		return OperationResult{Action: action, Target: target, Status: "succeeded", Message: result.Stdout, DurationMS: result.DurationMS}
	}
	return OperationResult{Action: action, Target: target, Status: "succeeded", Message: action + " completed", DurationMS: result.DurationMS}
}
