package errorTypes

import "errors"

type NoSuchPhoneBenchmarkError struct {
	Message string
}

type FullQueue struct {
	Message string
}

type DatabaseNetworkError struct {
	Message string
}

type AiNetworkError struct {
	Message string
}

type ErrorGettingURL struct {
	Message string
}

type ErrorCounterOverflowError struct {
	Message string
}

type SentimentAnalysisError struct {
	Message string
}

type CreatingNewAiClientError struct {
	Message string
}

type FailedAiInstructionError struct {
	Message string
}

type GettingURLError struct {
	Message string
}

type ParsingError struct {
	Message string
}

type MissingDocumentError struct {
	Message string
}

type GeneralDatabaseError struct {
	Message string
}

type InvalidConstIDStringError struct {
	Message string
}

type NoLastYearEquivalentError struct {
	Message string
}

type UninitializedDatabaseError struct {
	Message string
}

type CleanUpError struct {
	Message string
}

type InvalidDeviceError struct {
	Message string
}

func (e InvalidDeviceError) Error() string {
	return e.Message
}

func (e CleanUpError) Error() string {
	return e.Message
}

func (e NoLastYearEquivalentError) Error() string {
	return e.Message
}

func (e InvalidConstIDStringError) Error() string {
	return e.Message
}

func (e GeneralDatabaseError) Error() string {
	return e.Message
}

func (e MissingDocumentError) Error() string {
	return e.Message
}

func (e ParsingError) Error() string {
	return e.Message
}

func (e GettingURLError) Error() string {
	return e.Message
}

func (e FailedAiInstructionError) Error() string {
	return e.Message
}

func (e AiNetworkError) Error() string {
	return e.Message
}

func (e CreatingNewAiClientError) Error() string {
	return e.Message
}

func (e SentimentAnalysisError) Error() string {
	return e.Message
}

func (e NoSuchPhoneBenchmarkError) Error() string {
	return e.Message
}

func (e FullQueue) Error() string {
	return e.Message
}

func (e DatabaseNetworkError) Error() string {
	return e.Message
}

func (e ErrorGettingURL) Error() string {
	return e.Message
}

func (e ErrorCounterOverflowError) Error() string {
	return e.Message
}

func IsNoSuchPhoneBenchmarkError(err error) bool {
	var noPhoneErr NoSuchPhoneBenchmarkError
	return errors.As(err, &noPhoneErr)
}

func IsMissingDocumentError(err error) bool {
	var missingDocumentErr MissingDocumentError
	return errors.As(err, &missingDocumentErr)
}

func IsNoLastYearEquivalentError(err error) bool {
	var noLastYearErr NoLastYearEquivalentError
	return errors.As(err, &noLastYearErr)
}
