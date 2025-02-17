package aiAnalysis

import (
	language "cloud.google.com/go/language/apiv2"
	"cloud.google.com/go/language/apiv2/languagepb"
	"context"
	"encoding/json"
	"fmt"
	"github.com/ItaiHalperin/Device-Rec-API/dataTypes"
	"github.com/ItaiHalperin/Device-Rec-API/internal/errorHandling"
	"github.com/ItaiHalperin/Device-Rec-API/internal/errorMonitoring"
	"github.com/ItaiHalperin/Device-Rec-API/internal/errorTypes"
	"github.com/ItaiHalperin/Device-Rec-API/internal/helpers"
	"github.com/ItaiHalperin/Device-Rec-API/internal/parsingErrorLogger"
	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
	"log"
	"os"
	"strings"
	"time"
)

const (
	singleCoreScoreWeight = 0.6
	multiCoreScoreWeight  = 0.4

	densityScoreWeight     = 0.3
	nitsScoreWeight        = 0.3
	refreshRateScoreWeight = 0.3

	refreshRate60hzScore  = 0.10
	refreshRate90hzScore  = 0.5
	refreshRate120hzScore = 0.8
	refreshRate144hzScore = 1

	benchmarkScoreWeight = 50
	displayScoreWeight   = 20
	reviewScoreWeight    = 10
	batteryScoreWeight   = 10

	benchmarkEstimationOffset     = 5
	estimatedBenchmarkScoreWeight = 30
)

func AnalyseSentimentMagnitude(review string, ctrl *dataTypes.FlowControl) (float64, float64, error) {
	if ctrl.Ctx.Err() != nil {
		errorHandling.LogErrorToScreen(errorHandling.LogParams{
			DeviceName: "",
			Function:   "aiAnalysis.AnalyseSentimentMagnitude",
			ErrorMsg:   "",
			IsCtxError: true,
		}, ctrl.Ctx.Err())
		return 0, 0, ctrl.Ctx.Err()
	}

	apiKey := os.Getenv("GEN_AI_KEY")
	ctxForNewClient, cancelForNewClient := context.WithTimeout(ctrl.Ctx, time.Second*30)
	defer cancelForNewClient()

	client, err := language.NewClient(ctxForNewClient, option.WithAPIKey(apiKey))
	if err != nil {
		return 0, 0, err
	}
	defer func(client *language.Client) {
		if err = client.Close(); err != nil {
			errorHandling.LogErrorToScreen(errorHandling.LogParams{
				DeviceName: "",
				Function:   "aiAnalysis.AnalyseSentimentMagnitude",
				ErrorMsg:   "WARNING: Failed to close AI client",
				IsCtxError: false,
			}, ctrl.Ctx.Err())
			errorMonitoring.IncrementError(errorMonitoring.CleanUpError, ctrl)
		}
	}(client)
	ctxForAnalyze, cancelForAnalyze := context.WithTimeout(ctrl.Ctx, time.Second*30)
	defer cancelForAnalyze()
	resp, err := client.AnalyzeSentiment(ctxForAnalyze, &languagepb.AnalyzeSentimentRequest{
		Document: &languagepb.Document{
			Source: &languagepb.Document_Content{
				Content: review,
			},
			Type: languagepb.Document_PLAIN_TEXT,
		},
		EncodingType: languagepb.EncodingType_UTF8,
	})

	if err != nil {
		errorHandling.LogErrorToScreen(errorHandling.LogParams{
			DeviceName: "",
			Function:   "aiAnalysis.AnalyseSentimentMagnitude",
			ErrorMsg:   "failed to analyze sentiment",
			IsCtxError: false,
		}, ctrl.Ctx.Err())
		errorMonitoring.IncrementError(errorMonitoring.SentimentAnalysisError, ctrl)
		return 0, 0, errorTypes.NewSentimentAnalysisError("in aiAnalysis.AnalyseSentimentMagnitude failed to analyse sentiment")
	}

	verdictSentiment := resp.GetDocumentSentiment()
	return float64(verdictSentiment.Score), float64(verdictSentiment.Magnitude), nil
}

func IsCorrectWebpage(instruction, brandAndName, searchSnippet string, ctrl *dataTypes.FlowControl) (bool, error) {
	if ctrl.Ctx.Err() != nil {
		errorHandling.LogErrorToScreen(errorHandling.LogParams{
			DeviceName: "",
			Function:   "aiAnalysis.IsCorrectWebpage",
			ErrorMsg:   "",
			IsCtxError: true,
		}, ctrl.Ctx.Err())
		log.Printf("stopping aiAnlysis.IsCorrectWebpage: %v", ctrl.Ctx.Err())
		return false, ctrl.Ctx.Err()
	}

	isCorrect, err := GetBoolAIResponse(instruction, brandAndName+"\n"+"\""+searchSnippet+"\"", ctrl)
	if err != nil {
		log.Printf("in aiAnlysis.IsCorrectWebpage failed to get response from ai: %v", err)
		return false, err
	}

	return isCorrect, nil
}

func GetPriceCategory(brandAndName string, ctrl *dataTypes.FlowControl) (int, error) {
	if ctrl.Ctx.Err() != nil {
		log.Printf("stopping aiAnlysis.GetPriceCategory: %v", ctrl.Ctx.Err())
		return 0, ctrl.Ctx.Err()
	}

	response, err := GetStringAIResponse("You receive a phone model in the form \"[brand] [phone name]\""+
		" and you return a classification in terms of launch price range (LOW_END, LOW_MID_RANGE, HIGH_MID_RANGE, HIGH_END) and nothing further.",
		brandAndName, ctrl)
	if err != nil {
		log.Printf("in aiAnalysis.GetPriceCategory failed to gather price category: %v", err)
		return 0, err
	}

	response = strings.ReplaceAll(response, "\"", "")

	switch response {
	case "LOW_END":
		return dataTypes.LowEnd, nil
	case "LOW_MID_RANGE":
		return dataTypes.LowMidRange, nil
	case "HIGH_MID_RANGE":
		return dataTypes.HighMidRange, nil
	case "HIGH_END":
		return dataTypes.HighEnd, nil
	}

	errorMonitoring.IncrementError(errorMonitoring.FailedAiInstructionError, ctrl)
	parsingErrorLogger.LogErrorInJsonFile("in aiAnalysisAI.GetPriceCategory ai didn't return a price category", ctrl)
	return 0, errorTypes.NewFailedAiInstructionError("in aiAnalysisAI.GetPriceCategory ai didn't return a price category")
}

func GetBoolAIResponse(instruction, prompt string, ctrl *dataTypes.FlowControl) (bool, error) {
	if ctrl.Ctx.Err() != nil {
		log.Printf("stopping aiAnlysis.GetBoolAIResponse: %v", ctrl.Ctx.Err())
		return false, ctrl.Ctx.Err()
	}
	resp, err := getAiResponse[bool](instruction, prompt, ctrl)
	if err != nil {
		log.Printf("in aiAnlysis.GetBoolAIResponse failed to get response from ai: %v", err)
		return false, err
	}

	part := resp.Candidates[0].Content.Parts[0]
	if txt, ok := part.(genai.Text); ok {
		var boolResp bool
		if err = json.Unmarshal([]byte(txt), &boolResp); err != nil {
			log.Printf("in aiAnlysis.GetBoolAIResponse failed to parse response from ai: %v", err)
			parsingErrorLogger.LogErrorInJsonFile(fmt.Sprintf("in aiAnlysis.GetBoolAIResponse failed to parse response from ai: %v", err), ctrl)
			errorMonitoring.IncrementError(errorMonitoring.ParsingError, ctrl)
			return false, errorTypes.NewParsingError(fmt.Sprintf("in aiAnlysis.GetBoolAIResponse failed to parse response from ai: %v", err))
		}
		return boolResp, nil
	}
	log.Printf("in aiAnlysis.GetBoolAIResponse failed to get text from resp: %v", err)
	return false, err
}
func GetStringAIResponse(instruction, prompt string, ctrl *dataTypes.FlowControl) (string, error) {
	if ctrl.Ctx.Err() != nil {
		log.Printf("stopping aiAnlysis.GetStringAIResponse: %v", ctrl.Ctx.Err())
		return "", ctrl.Ctx.Err()
	}
	resp, err := getAiResponse[string](instruction, prompt, ctrl)
	if err != nil {
		log.Printf("in aiAnlysis.GetStringAIResponse failed to get response from ai: %v", err)
		return "", err
	}

	part := resp.Candidates[0].Content.Parts[0]
	if txt, ok := part.(genai.Text); ok {
		stringResp := string(txt)
		return stringResp, nil
	}
	log.Printf("in aiAnlysis.GetStringAIResponse failed to parse response from ai: %v", err)
	parsingErrorLogger.LogErrorInJsonFile(fmt.Sprintf("in aiAnlysis.GetStringAIResponse failed to parse response from ai: %v", err), ctrl)
	errorMonitoring.IncrementError(errorMonitoring.ParsingError, ctrl)
	return "", errorTypes.NewParsingError(fmt.Sprintf("in aiAnlysis.GetStringAIResponse failed to parse response from ai: %v", err))
}
func GetIntAIResponse(instruction, prompt string, ctrl *dataTypes.FlowControl) (int, error) {
	if ctrl.Ctx.Err() != nil {
		log.Printf("stopping aiAnlysis.GetIntAIResponse: %v", ctrl.Ctx.Err())
		return 0, ctrl.Ctx.Err()
	}
	resp, err := GetStringAIResponse(instruction, prompt, ctrl)
	if err != nil {
		log.Printf("in aiAnlysis.GetIntAIResponse failed to get string response from ai: %v", err)
		return 0, err
	}

	floatResp, err := helpers.ExtractFloat(resp)
	if err != nil {
		log.Printf("in aiAnlysis.GetIntAIResponse failed to parse response (%s) into int from ai: %v", resp, err)
		parsingErrorLogger.LogErrorInJsonFile(fmt.Sprintf("in aiAnlysis.GetIntAIResponse failed to parse response (%s) into int from ai: %v", resp, err), ctrl)
		errorMonitoring.IncrementError(errorMonitoring.ParsingError, ctrl)
		return 0, errorTypes.NewParsingError(fmt.Sprintf("in aiAnlysis.GetIntAIResponse failed to parse response (%s) into int from ai: %v", resp, err))
	}

	return int(floatResp), nil
}

func getAiResponse[T any](instruction, prompt string, ctrl *dataTypes.FlowControl) (*genai.GenerateContentResponse, error) {
	if ctrl.Ctx.Err() != nil {
		log.Printf("stopping aiAnlysis.getAiResponse: %v", ctrl.Ctx.Err())
		return nil, ctrl.Ctx.Err()
	}

	apiKey := os.Getenv("GEN_AI_KEY")
	ctxForNewClient, cancelForNewClient := context.WithTimeout(ctrl.Ctx, time.Second*30)
	defer cancelForNewClient()
	client, err := genai.NewClient(ctxForNewClient, option.WithAPIKey(apiKey))
	if err != nil {
		log.Printf("WARNING: Failed to create AI client: %v", err)
		errorMonitoring.IncrementError(errorMonitoring.CreatingNewAiClientError, ctrl)
		return nil, err
	}
	defer func(client *genai.Client) {
		err := client.Close()
		if err != nil {
			log.Printf("WARNING: Failed to close AI client: %v", err)
			errorMonitoring.IncrementError(errorMonitoring.CleanUpError, ctrl)
		}
	}(client)

	model := client.GenerativeModel("gemini-1.5-flash")

	model.SetTemperature(0)
	model.SetTopK(1)
	model.SetTopP(0.95)
	model.ResponseMIMEType = "application/json"
	setResponseSchema[T](model)
	model.SystemInstruction = &genai.Content{
		Parts: []genai.Part{genai.Text(instruction)},
	}

	// model.SafetySettings = Adjust safety settings
	// See https://ai.google.dev/gemini-api/docs/safety-settings
	ctxForGenerate, cancelForGenerate := context.WithTimeout(ctrl.Ctx, time.Minute)
	defer cancelForGenerate()
	resp, err := model.GenerateContent(ctxForGenerate, genai.Text(prompt))

	if err != nil {
		log.Printf("in aiAnalysis.getAiResponse failed to message AI client: %v", err)
		errorMonitoring.IncrementError(errorMonitoring.AiNetworkError, ctrl)
		return nil, err
	}

	return resp, nil
}

func setResponseSchema[T any](model *genai.GenerativeModel) {
	var t T
	switch any(t).(type) {
	case bool:
		model.ResponseSchema = &genai.Schema{
			Type: genai.TypeBoolean,
		}
	case int:
		model.ResponseSchema = &genai.Schema{
			Type: genai.TypeInteger,
		}
	case string:
		model.ResponseSchema = &genai.Schema{
			Type: genai.TypeString,
		}
	}
}

type scoreWeights struct {
	benchmark float64
	display   float64
	review    float64
	battery   float64
}

func GetFinalScore(newMinMaxMagnitudeSentiment dataTypes.MinMaxValues, device *dataTypes.Device, scoresType int) float64 {
	normalizedSingleCoreScore := helpers.CalculateNormalizedValue(newMinMaxMagnitudeSentiment.SingleCoreScore.Min, newMinMaxMagnitudeSentiment.SingleCoreScore.Max, device.Benchmark.SingleCoreScore)
	normalizedMultiCoreScore := helpers.CalculateNormalizedValue(newMinMaxMagnitudeSentiment.MultiCoreScore.Min, newMinMaxMagnitudeSentiment.MultiCoreScore.Max, device.Benchmark.MultiCoreScore)
	normalizedPixelDensity := helpers.CalculateNormalizedValue(newMinMaxMagnitudeSentiment.PixelDensity.Min, newMinMaxMagnitudeSentiment.PixelDensity.Max, device.Specs.PixelDensity)
	normalizedNitsScore := helpers.CalculateNormalizedValue(newMinMaxMagnitudeSentiment.Nits.Min, newMinMaxMagnitudeSentiment.Nits.Max, float64(device.Specs.Nits))
	normalizedBatteryScore := helpers.CalculateNormalizedValue(newMinMaxMagnitudeSentiment.BatteryCapacity.Min, newMinMaxMagnitudeSentiment.BatteryCapacity.Max, device.Specs.BatteryCapacity)

	var refreshRateScore float64
	switch device.Specs.RefreshRate {
	case 60:
		refreshRateScore = refreshRate60hzScore
	case 90:
		refreshRateScore = refreshRate90hzScore
	case 120:
		refreshRateScore = refreshRate120hzScore
	case 144:
		refreshRateScore = refreshRate144hzScore
	}

	normalizedBenchmarkScore := singleCoreScoreWeight*normalizedSingleCoreScore + multiCoreScoreWeight*normalizedMultiCoreScore
	normalizedDisplayScore := normalizedNitsScore*nitsScoreWeight*normalizedPixelDensity*densityScoreWeight + refreshRateScore*refreshRateScoreWeight

	var weights scoreWeights
	if device.Benchmark.IsEstimatedBenchmark {
		weights.benchmark = estimatedBenchmarkScoreWeight
		weights.battery = batteryScoreWeight + benchmarkEstimationOffset
		weights.display = displayScoreWeight + benchmarkEstimationOffset
		weights.review = reviewScoreWeight + benchmarkEstimationOffset
	} else {
		weights.battery = batteryScoreWeight
		weights.display = displayScoreWeight
		weights.review = reviewScoreWeight
		weights.benchmark = benchmarkScoreWeight
	}
	if scoresType == dataTypes.UnvalidatedScores {
		return weights.display*normalizedDisplayScore + weights.battery*normalizedBatteryScore +
			+weights.benchmark*normalizedBenchmarkScore +
			weights.review*device.Review.UnvalidatedReviewScore
	}
	return weights.display*normalizedDisplayScore + weights.battery*normalizedBatteryScore +
		+weights.benchmark*normalizedBenchmarkScore +
		weights.review*device.Review.ValidatedReviewScore
}
