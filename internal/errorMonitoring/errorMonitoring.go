package errorMonitoring

import (
	"DeviceRecommendationProject/internal/dataTypes"
	"DeviceRecommendationProject/internal/mutexGetter"
	"encoding/json"
	"log"
	"os"
)

const (
	CleanUpErrorsMaxAmount              = 10
	SentimentAnalysisErrorsMaxAmount    = 3
	CreatingNewAiClientErrorsMaxAmount  = 3
	AiNetworkErrorsMaxAmount            = 3
	FailedAiInstructionErrorsMaxAmount  = 1
	GettingURLErrorsMaxAmount           = 5
	GettingDocumentErrorsMaxAmount      = 5
	ParsingErrorsMaxAmount              = 5
	MissingDocumentErrorsMaxAmount      = 1
	DatabaseNetworkErrorsMaxAmount      = 3
	GeneralDatabaseErrorsMaxAmount      = 1
	InvalidConstIDStringErrorsMaxAmount = 1
)

const (
	CleanUpError              = "clean_up_error"
	SentimentAnalysisError    = "sentiment_analysis_error"
	CreatingNewAiClientError  = "creating_new_ai_client_error"
	AiNetworkError            = "ai_network_error"
	FailedAiInstructionError  = "failed_ai_instruction_error"
	GettingURLError           = "getting_url_error"
	GettingDocumentError      = "getting_document_error"
	ParsingError              = "parsing_error"
	MissingDocumentError      = "missing_document_error"
	DatabaseNetworkError      = "database_network_error"
	GeneralDatabaseError      = "general_database_error"
	InvalidConstIDStringError = "invalid_const_id_string_error"
)

const (
	errorCountersPath = "internal/errorMonitoring/errorCounters.json"
)

func IncrementError(errorType string, ctrl *dataTypes.FlowControl) {
	if ctrl.Ctx.Err() != nil {
		log.Printf("stopping mongoDatabase.IncrementError: %v", ctrl.Ctx.Err())
		return
	}

	mutexGetter.GetMutex().Lock()
	defer mutexGetter.GetMutex().Unlock()

	log.Println()
	errorCountersJson, err := os.ReadFile(errorCountersPath)
	if err != nil {
		log.Println("in errorMonitoring.IncrementCleanUpErrors failed to read error counters file: ", err)
		ctrl.StopOnTooManyErrorsChannel <- struct{}{}
	}
	var errorCounters dataTypes.ErrorCounters
	err = json.Unmarshal(errorCountersJson, &errorCounters)
	if err != nil {
		log.Println("in errorMonitoring.IncrementCleanUpErrors failed to unmarshal error counters file: ", err)
		ctrl.StopOnTooManyErrorsChannel <- struct{}{}
	}

	switch errorType {
	case CleanUpError:
		errorCounters.CleanUpErrors++
		checkErrorThreshold(errorCounters.CleanUpErrors, CleanUpErrorsMaxAmount, ctrl)
	case SentimentAnalysisError:
		errorCounters.SentimentAnalysisErrors++
		checkErrorThreshold(errorCounters.SentimentAnalysisErrors, SentimentAnalysisErrorsMaxAmount, ctrl)
	case CreatingNewAiClientError:
		errorCounters.CreatingNewAiClientErrors++
		checkErrorThreshold(errorCounters.CreatingNewAiClientErrors, CreatingNewAiClientErrorsMaxAmount, ctrl)
	case AiNetworkError:
		errorCounters.AiNetworkErrors++
		checkErrorThreshold(errorCounters.AiNetworkErrors, AiNetworkErrorsMaxAmount, ctrl)
	case FailedAiInstructionError:
		errorCounters.FailedAiInstructionErrors++
		checkErrorThreshold(errorCounters.FailedAiInstructionErrors, FailedAiInstructionErrorsMaxAmount, ctrl)
	case GettingURLError:
		errorCounters.GettingURLErrors++
		checkErrorThreshold(errorCounters.GettingURLErrors, GettingURLErrorsMaxAmount, ctrl)
	case GettingDocumentError:
		errorCounters.GettingDocumentErrors++
		checkErrorThreshold(errorCounters.GettingDocumentErrors, GettingDocumentErrorsMaxAmount, ctrl)
	case ParsingError:
		errorCounters.ParsingErrors++
		checkErrorThreshold(errorCounters.ParsingErrors, ParsingErrorsMaxAmount, ctrl)
	case MissingDocumentError:
		errorCounters.MissingDocumentErrors++
		checkErrorThreshold(errorCounters.MissingDocumentErrors, MissingDocumentErrorsMaxAmount, ctrl)
	case DatabaseNetworkError:
		errorCounters.DatabaseNetworkErrors++
		checkErrorThreshold(errorCounters.DatabaseNetworkErrors, DatabaseNetworkErrorsMaxAmount, ctrl)
	case GeneralDatabaseError:
		errorCounters.GeneralDatabaseErrors++
		checkErrorThreshold(errorCounters.GeneralDatabaseErrors, GeneralDatabaseErrorsMaxAmount, ctrl)
	case InvalidConstIDStringError:
		errorCounters.InvalidConstIDStringErrors++
		checkErrorThreshold(errorCounters.InvalidConstIDStringErrors, InvalidConstIDStringErrorsMaxAmount, ctrl)
	}

	updatedJSON, _ := json.MarshalIndent(errorCounters, "", "  ")
	err = os.WriteFile(errorCountersPath, updatedJSON, 0644)
	if err != nil {
		log.Println("in errorMonitoring.IncrementCleanUpErrors failed to rewrite error file: ", err)
		ctrl.StopOnTooManyErrorsChannel <- struct{}{}
	}
	log.Printf("in errors file incremented %v", errorType)
}

func checkErrorThreshold(currentErrors int, maxErrors int, ctrl *dataTypes.FlowControl) {
	if currentErrors > maxErrors {
		ctrl.StopOnTooManyErrorsChannel <- struct{}{}
	}
}

func ResetErrorCounters(ctrl *dataTypes.FlowControl) error {
	var data dataTypes.ErrorCounters
	updatedJSON, _ := json.MarshalIndent(data, "", "  ")
	err := os.WriteFile(errorCountersPath, updatedJSON, 0644)
	if err != nil {
		log.Println("in errorMonitoring.ResetErrorCounters failed to rewrite error file: ", err)
		ctrl.StopOnTooManyErrorsChannel <- struct{}{}
		return err
	}
	return nil
}
