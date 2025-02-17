package parsingErrorLogger

import (
	"encoding/json"
	"github.com/ItaiHalperin/Device-Rec-API/internal/dataTypes"
	"github.com/ItaiHalperin/Device-Rec-API/internal/helpers"
	"log"
	"os"
	"time"
)

const (
	parsingErrorLogsPath = "internal/parsingErrorLogger/parsingErrorLogs.json"
)

type parsingErrorLogsFile struct {
	ErrorLogs []dataTypes.ParsingErrorLog `json:"logs"`
}

func LogErrorInJsonFile(message string, ctrl *dataTypes.FlowControl) {
	errLog := dataTypes.ParsingErrorLog{
		Time:    time.Now(),
		Message: message,
		Trace:   helpers.GetStackTrace(1),
	}
	parsingErrorLogsJson, err := os.ReadFile(parsingErrorLogsPath)
	if err != nil {
		log.Println("in parsingErrorLogger.LogErrorInJsonFile failed to read parsing error logs file: ", err)
		ctrl.StopOnTooManyErrorsChannel <- struct{}{}
	}
	var parsingErrorLogs parsingErrorLogsFile
	err = json.Unmarshal(parsingErrorLogsJson, &parsingErrorLogs)
	if err != nil {
		log.Println("in parsingErrorLogger.LogErrorInJsonFile failed to unmarshal parsing error logs file: ", err)
		ctrl.StopOnTooManyErrorsChannel <- struct{}{}
	}

	parsingErrorLogs.ErrorLogs = append(parsingErrorLogs.ErrorLogs, errLog)

	updatedJSON, _ := json.MarshalIndent(parsingErrorLogs, "", "  ")
	err = os.WriteFile(parsingErrorLogsPath, updatedJSON, 0644)
	if err != nil {
		log.Println("in errorMonitoring.IncrementCleanUpErrors failed to rewrite error file: ", err)
		ctrl.StopOnTooManyErrorsChannel <- struct{}{}
	}

	log.Printf("in parsing error logs file added log %v", errLog)
}
