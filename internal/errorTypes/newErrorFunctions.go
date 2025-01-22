package errorTypes

func NewSentimentAnalysisError(message string) SentimentAnalysisError {
	return SentimentAnalysisError{message}
}

func NewErrorsCounterOverflowError(message string) ErrorCounterOverflowError {
	return ErrorCounterOverflowError{message}
}

func NewNoSuchPhoneBenchmark(message string) NoSuchPhoneBenchmarkError {
	return NoSuchPhoneBenchmarkError{message}
}

func NewErrorGettingURL(message string) ErrorGettingURL {
	return ErrorGettingURL{message}
}

func NewFullQueue(message string) FullQueue {
	return FullQueue{message}
}

func NewCreatingNewAiClientError(message string) CreatingNewAiClientError {
	return CreatingNewAiClientError{message}
}

func NewFailedAiInstructionError(message string) FailedAiInstructionError {
	return FailedAiInstructionError{message}
}

func NewParsingError(message string) ParsingError {
	return ParsingError{message}
}

func NewNoSuchPhoneBenchmarkError(message string) NoSuchPhoneBenchmarkError {
	return NoSuchPhoneBenchmarkError{message}
}

func NewMissingDocumentError(message string) MissingDocumentError {
	return MissingDocumentError{message}
}

func NewGeneralDatabaseError(message string) GeneralDatabaseError {
	return GeneralDatabaseError{message}
}

func NewInvalidConstIDStringError(message string) InvalidConstIDStringError {
	return InvalidConstIDStringError{message}
}

func NewNoLastYearEquivalentError(message string) NoLastYearEquivalentError {
	return NoLastYearEquivalentError{message}
}

func NewDatabaseNetworkError(message string) DatabaseNetworkError {
	return DatabaseNetworkError{message}
}

func NewCleanUpError(message string) CleanUpError {
	return CleanUpError{message}
}

func NewInvalidDeviceError(message string) InvalidDeviceError {
	return InvalidDeviceError{message}
}
