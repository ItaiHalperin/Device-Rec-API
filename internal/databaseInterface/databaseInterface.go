package databaseInterface

import (
	"DeviceRecommendationProject/internal/dataTypes"
)

type DatabaseInterface interface {
	UploadDevice(*dataTypes.Device, dataTypes.MinMaxValues, *dataTypes.FlowControl) error
	GetValidatedAndUnvalidatedMinMaxValues(*dataTypes.FlowControl) (dataTypes.ValidatedAndUnvalidatedMinMaxValues, error)
	NormalizeUnvalidatedScores(dataTypes.MinMaxValues, *dataTypes.FlowControl) error
	Connect(*dataTypes.FlowControl) error
	Disconnect(*dataTypes.FlowControl) error
	IsUp(*dataTypes.FlowControl) bool
	GetLastYearEquivalentBenchmarkScores(*dataTypes.Device, *dataTypes.FlowControl) (float64, float64, error)
	UploadDeviceBatch([]*dataTypes.Device, dataTypes.MinMaxValues, *dataTypes.FlowControl) error
	Dequeue(ctrl *dataTypes.FlowControl) (dataTypes.DeviceInQueue, error)
	EnqueueDeviceBatch(map[string][]string, *dataTypes.FlowControl) error
	ReestimateBenchmarks(*dataTypes.FlowControl) error
	ResetDatabase(*dataTypes.FlowControl) error
	SetLastYearEquivalentBenchmarkScores(*dataTypes.Device, *dataTypes.FlowControl) error
	GetTop3(*dataTypes.Filters, *dataTypes.FlowControl) ([]dataTypes.Device, error)
}
