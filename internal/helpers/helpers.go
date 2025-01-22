package helpers

import (
	"SimpleWeb/internal/dataTypes"
	"SimpleWeb/internal/errorMonitoring"
	"SimpleWeb/internal/errorTypes"
	"errors"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"io"
	"log"
	"math/rand"
	"net/http"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

func DecrementNumberInString(input string) (string, error) {
	// Regular expression to find the first number in the string
	re := regexp.MustCompile(`\d+`)
	numberStr := re.FindString(input)

	if numberStr == "" {
		// Return an error if no number is found
		return "", errors.New("no number found in the string")
	}

	// Convert the found number to an integer
	number, err := strconv.Atoi(numberStr)
	if err != nil {
		return "", err
	}

	// Decrease the number by 1
	decrementedNumber := number - 1

	// Replace the original number in the string with the decremented value
	result := re.ReplaceAllString(input, strconv.Itoa(decrementedNumber))
	return result, nil
}

func GetBeforeSubstring(s, sub string) string {
	index := strings.Index(s, sub)
	if index == -1 {
		return s
	}
	return s[:index]
}

func GetAfterSubstring(s, sub string) string {
	index := strings.Index(s, sub)
	if index == -1 {
		return s
	}
	return s[index+len(sub):]
}

func GetDocumentByURL(url string, ctrl *dataTypes.FlowControl) (*goquery.Document, error) {
	if ctrl.Ctx.Err() != nil {
		log.Printf("stopping aiAnlysis.GetDocumentByURL: %v", ctrl.Ctx.Err())
		return nil, ctrl.Ctx.Err()
	}

	resp, err := GetRespByURL(url, ctrl)
	if err != nil {
		log.Printf("helpers.GetDocumentByURL got bad status code")
		return nil, err
	}

	defer func(Body io.ReadCloser) {
		if err := Body.Close(); err != nil {
			log.Printf("WARNING: Failed to close HTML reader: %v", err)
			errorMonitoring.IncrementError(errorMonitoring.CleanUpError, ctrl)
		}
	}(resp.Body)

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		log.Printf("helpers.GetDocumentByURL failed to get document from response: %v", err)
		errorMonitoring.IncrementError(errorMonitoring.GettingDocumentError, ctrl)
		return nil, errorTypes.NewErrorGettingURL("in aiAnalysis.GetDocumentByURL failed to get document from reader")
	}

	return doc, nil
}

func GetRespByURL(url string, ctrl *dataTypes.FlowControl) (*http.Response, error) {
	if ctrl.Ctx.Err() != nil {
		log.Printf("stopping aiAnlysis.GetRespByURL: %v", ctrl.Ctx.Err())
		return nil, ctrl.Ctx.Err()
	}

	url = strings.ReplaceAll(url, " ", "+")
	resp, err := http.Get(url)
	if err != nil {
		errorMonitoring.IncrementError(errorMonitoring.GettingURLError, ctrl)
		return nil, errorTypes.NewErrorGettingURL("in aiAnalysis.GetDocumentByURL error getting HTML")
	}

	if resp.StatusCode != 200 {
		log.Println("helpers.GetRespByURL got bad status code")
		errorMonitoring.IncrementError(errorMonitoring.GettingURLError, ctrl)
		return nil, errorTypes.NewErrorGettingURL(fmt.Sprintf("helpers.GetRespByURL got bad status code: %d %s", resp.StatusCode, resp.Status))
	}
	return resp, nil
}

func GetSubMap(original map[string][]string, count int) map[string][]string {
	result := make(map[string][]string)

	// Convert map keys to slice for random selection
	keys := make([]string, 0, len(original))
	for k := range original {
		keys = append(keys, k)
	}

	// Handle case where requested count is larger than map size
	if count > len(keys) {
		count = len(keys)
	}

	// Fisher-Yates shuffle for the first 'count' elements (no repetition)
	for i := 0; i < count; i++ {
		j := rand.Intn(len(keys)-i) + i
		keys[i], keys[j] = keys[j], keys[i]
		result[keys[i]] = original[keys[i]]
	}

	return result
}

func ExtractFloat(s string) (float64, error) {
	var numStr string
	dotSeen := false

	for _, r := range s {
		if unicode.IsDigit(r) {
			numStr += string(r)
		} else if r == '.' && !dotSeen {
			numStr += string(r)
			dotSeen = true
		} else if r == ',' {
			continue
		} else if len(numStr) > 0 {
			break
		}
	}

	if numStr == "" {
		log.Println("in helpers.ExtractFloat no float found in string")
		return 0, errorTypes.NewParsingError("in helpers.ExtractFloat no float found in string")
	}

	num, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		log.Println("in helpers.ExtractFloat no float found in string")
		return 0, errorTypes.NewParsingError(fmt.Sprintf("in helpers.ExtractFloat error parsing float: %v", err))
	}
	return num, nil
}

func DoesNameContainExcludedKeywords(name string, excluded []string) bool {
	nameLower := strings.ToLower(name)
	for _, keyword := range excluded {
		if strings.Contains(nameLower, strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}

func GetDefaultMinMax() dataTypes.MinMaxValues {
	return dataTypes.MinMaxValues{Sentiment: dataTypes.MinMaxFloat{Min: 1e+308},
		Magnitude:       dataTypes.MinMaxFloat{Min: 1e+308},
		SingleCoreScore: dataTypes.MinMaxFloat{Min: 1e+308},
		MultiCoreScore:  dataTypes.MinMaxFloat{Min: 1e+308},
		BatteryCapacity: dataTypes.MinMaxFloat{Min: 1e+308},
		PixelDensity:    dataTypes.MinMaxFloat{Min: 1e+308},
		Nits:            dataTypes.MinMaxFloat{Min: 1e+308}}
}

func GetKeys(m map[string][]string) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	return keys
}

func SortDevicesByDate(devices []dataTypes.Device) {
	sort.Slice(devices, func(i, j int) bool {
		timeI := devices[i].Specs.ReleaseDate
		timeJ := devices[j].Specs.ReleaseDate

		return timeI.Before(timeJ)
	})
}

func GetStackTrace(skip int) string {
	var sb strings.Builder
	pc := make([]uintptr, 10)
	n := runtime.Callers(2+skip, pc)
	frames := runtime.CallersFrames(pc[:n])

	for {
		frame, more := frames.Next()
		sb.WriteString(fmt.Sprintf("%s\n\t%s:%d\n", frame.Function, frame.File, frame.Line))
		if !more {
			break
		}
	}

	return sb.String()
}
