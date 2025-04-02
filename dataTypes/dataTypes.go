package dataTypes

import (
	"context"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"
)

const (
	ValidatedScores = iota
	UnvalidatedScores
)
const (
	LowEnd = iota
	LowMidRange
	HighMidRange
	HighEnd
)

const EarliestYearBound = 2019

type Year struct {
	ID         primitive.ObjectID   `bson:"_id"`
	YearNumber int                  `bson:"year-number"`
	Months     []primitive.ObjectID `bson:"months"`
}

type Device struct {
	ID                    primitive.ObjectID `bson:"_id"`
	Month                 primitive.ObjectID `bson:"month"`
	Year                  primitive.ObjectID `bson:"year"`
	Brand                 string             `bson:"brand"`
	Name                  string             `bson:"name"`
	Specs                 Specifications     `bson:"specs"`
	Review                ReviewData         `bson:"review"`
	Benchmark             BenchmarkScores    `bson:"benchmark"`
	ValidatedFinalScore   float64            `bson:"validated-final-score"`
	UnvalidatedFinalScore float64            `bson:"unvalidated-final-score"`
	RealPrice             int                `bson:"real-price"`
	PriceCategory         int                `bson:"price-category"`
	Image                 string             `bson:"image"`
}

type BenchmarkScores struct {
	IsEstimatedBenchmark bool    `bson:"is-estimated-benchmark"`
	MultiCoreScore       float64 `bson:"multi-core-score"`
	SingleCoreScore      float64 `bson:"single-core-score"`
}

type ReviewData struct {
	ReviewMagnitude        float64 `bson:"review-magnitude"`
	ReviewSentiment        float64 `bson:"review-sentiment"`
	ValidatedReviewScore   float64 `bson:"validated-review-score"`
	UnvalidatedReviewScore float64 `bson:"unvalidated-review-score"`
}

type Specifications struct {
	ReleaseDate        time.Time `bson:"release-date"`
	BatteryCapacity    float64   `bson:"battery-capacity"`
	DisplaySize        float64   `bson:"display-size"`
	DisplayResolution  string    `bson:"display-resolution"`
	MainCamerasSetup   string    `bson:"main-cameras-setup"`
	SelfieCamerasSetup string    `bson:"selfie-cameras-setup"`
	PixelDensity       float64   `bson:"pixel-density"`
	RefreshRate        int       `bson:"refresh-rate"`
	Nits               int       `bson:"nits"`
}

type Month struct {
	ID          primitive.ObjectID   `bson:"_id"`
	MonthNumber int                  `bson:"month-number"`
	Year        primitive.ObjectID   `bson:"year"`
	Devices     []primitive.ObjectID `bson:"devices"`
}

type MinMaxFloat struct {
	Min float64 `bson:"min"`
	Max float64 `bson:"max"`
}

type MinMaxInt struct {
	Min int
	Max int
}

type ValidatedAndUnvalidatedMinMaxValues struct {
	Validated   MinMaxValues `bson:"validated"`
	Unvalidated MinMaxValues `bson:"unvalidated"`
}

type MinMaxValues struct {
	Sentiment       MinMaxFloat `bson:"sentiment"`
	Magnitude       MinMaxFloat `bson:"magnitude"`
	SingleCoreScore MinMaxFloat `bson:"single-core-score"`
	MultiCoreScore  MinMaxFloat `bson:"multi-core-score"`
	BatteryCapacity MinMaxFloat `bson:"battery-capacity"`
	PixelDensity    MinMaxFloat `bson:"pixel-density"`
	Nits            MinMaxFloat `bson:"nits"`
}

type DeviceInQueue struct {
	Name   string `bson:"name"`
	Image  string `bson:"image"`
	Detail string `bson:"detail"`
}

type ErrorCounters struct {
	CleanUpErrors              int `json:"clean_up_errors"`
	SentimentAnalysisErrors    int `json:"sentiment_analysis_errors"`
	CreatingNewAiClientErrors  int `json:"creating_new_ai_client_errors"`
	AiNetworkErrors            int `json:"ai_network_errors"`
	FailedAiInstructionErrors  int `json:"failed_ai_instruction_errors"`
	GettingURLErrors           int `json:"getting_url_errors"`
	GettingDocumentErrors      int `json:"getting_document_errors"`
	ParsingErrors              int `json:"parsing_errors"`
	MissingDocumentErrors      int `json:"missing_document_errors"`
	DatabaseNetworkErrors      int `json:"database_network_errors"`
	GeneralDatabaseErrors      int `json:"general_database_errors"`
	InvalidConstIDStringErrors int `json:"invalid_const_id_string_errors"`
}

type YearIDsDocument struct {
	YearIDs []primitive.ObjectID `bson:"year-ids"`
}

type DeviceIDsDocument struct {
	DeviceIDs []primitive.ObjectID `bson:"device-ids"`
}

type FlowControl struct {
	Ctx                        context.Context
	StopOnTooManyErrorsChannel chan<- struct{}
}

type ParsingErrorLog struct {
	Time    time.Time `json:"time"`
	Message string    `json:"message"`
	Trace   string    `json:"trace"`
}

type MinMaxDate struct {
	MinDate time.Time
	MaxDate time.Time
}

type Filters struct {
	Price       MinMaxInt
	DisplaySize MinMaxFloat
	RefreshRate MinMaxInt
	Brands      []string
}

type ValidationFlag struct {
	IsUnfinishedValidation bool `bson:"is-unfinished-validation"`
}
