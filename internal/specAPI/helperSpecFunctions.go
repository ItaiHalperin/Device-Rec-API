package specAPI

import (
	"fmt"
	"github.com/ItaiHalperin/Device-Rec-API/internal/aiAnalysis"
	"github.com/ItaiHalperin/Device-Rec-API/internal/dataTypes"
	"github.com/ItaiHalperin/Device-Rec-API/internal/errorMonitoring"
	"github.com/ItaiHalperin/Device-Rec-API/internal/errorTypes"
	"github.com/ItaiHalperin/Device-Rec-API/internal/helpers"
	"github.com/ItaiHalperin/Device-Rec-API/internal/parsingErrorLogger"
	"log"
	"strconv"
	"strings"
	"time"
)

func setReleaseDate(deviceName, deviceURL string, curSpecs *dataTypes.Specifications, releaseDateString string, ctrl *dataTypes.FlowControl) error {
	if strings.ToLower(releaseDateString) == "cancelled" {
		log.Printf("in helperSpecFunctions.setReleaseDate cancelled device: %v", deviceName)
		return errorTypes.NewInvalidDeviceError(fmt.Sprintf("in helperSpecFunctions.setReleaseDate canceled device: %v", deviceName))
	}
	firstLayout := "2006, January 02"
	parsedDate, err := time.Parse(firstLayout, helpers.GetAfterSubstring(releaseDateString, "Released "))

	if err != nil {
		secondLayout := "2006, January"
		parsedDate, err = time.Parse(secondLayout, helpers.GetAfterSubstring(releaseDateString, "Released "))
		if err != nil {
			errMsg := fmt.Sprintf("in helperSpecFunctions.setReleaseDate (device: %v, url: %v)\nerror parsing date: %v", deviceName, deviceURL, err)
			log.Printf(errMsg)
			parsingErrorLogger.LogErrorInJsonFile(errMsg, ctrl)
			errorMonitoring.IncrementError(errorMonitoring.ParsingError, ctrl)
			return errorTypes.NewParsingError(errMsg)
		}
	}

	if parsedDate.Year() < dataTypes.EarliestYearBound {
		errMsg := fmt.Sprintf("in helperSpecFunctions.setReleaseDate (device: %v, url: %v)\nancient device, released in: %v",
			deviceName, deviceURL, parsedDate.Year())
		return errorTypes.NewInvalidDeviceError(errMsg)
	}
	curSpecs.ReleaseDate = parsedDate
	return nil
}

func extractCameraSetup(deviceName, deviceURL string, specsByKeys []SpecByKey, ctrl *dataTypes.FlowControl) (string, error) {
	for _, detail := range specsByKeys {
		switch detail.Key {
		case "Single", "Dual", "Triple", "Quad":
			return detail.Key, nil
		}
	}
	errMsg := fmt.Sprintf("in helperSpecFunctions.extractCameraSetup (device: %v, url: %v)\nfailed to gather camera setup", deviceName, deviceURL)
	log.Printf(errMsg)
	parsingErrorLogger.LogErrorInJsonFile(errMsg, ctrl)
	errorMonitoring.IncrementError(errorMonitoring.ParsingError, ctrl)
	return "", errorTypes.NewParsingError(errMsg)
}

func setDisplayDetails(deviceName, deviceURL string, curSpecs *dataTypes.Specifications, specsByKeys []SpecByKey, ctrl *dataTypes.FlowControl) error {
	numOfSpecsCollected := 0
	numOfSpecsExpected := 4
	for _, detail := range specsByKeys {
		switch detail.Key {
		case "Size":
			displaySize, err := extractDisplaySize(deviceName, deviceURL, detail.Val, ctrl)
			if err != nil {
				log.Printf("in helperSpecFunctions.setDisplayDetails error extracting display size: %v", err)
				return err
			}
			curSpecs.DisplaySize = displaySize
			numOfSpecsCollected++
		case "Resolution":
			displayResolution, err := extractDisplayResolution(deviceName, deviceURL, detail.Val, ctrl)
			if err != nil {
				log.Printf("in helperSpecFunctions.setDisplayDetails error extracting display resolution: %v", err)
				return err
			}
			curSpecs.DisplayResolution = displayResolution
			numOfSpecsCollected++
		case "Type":
			curSpecs.RefreshRate = extractRefreshRate(detail.Val)
			numOfSpecsCollected++
			nits, err := extractNits(deviceName, deviceURL, detail.Val)
			if err != nil {
				nits, err = getNitsFromAi(deviceName, deviceURL, ctrl)
				if err != nil {
					log.Printf("in helperSpecFunctions.setDisplayDetails error extracting display nits (both ai and api): %v", err)
					return err
				}
			}
			curSpecs.Nits = nits
			numOfSpecsCollected++
		}
	}
	if numOfSpecsCollected == numOfSpecsExpected {
		return nil
	}
	log.Printf("in helperSpecFunctions.setDisplayDetails failed to all display details")
	parsingErrorLogger.LogErrorInJsonFile("in helperSpecFunctions.setDisplayDetails failed to all "+
		"display details", ctrl)
	errorMonitoring.IncrementError(errorMonitoring.ParsingError, ctrl)
	return errorTypes.NewParsingError("in helperSpecFunctions.setDisplayDetails failed to all display details")
}

func getNitsFromAi(deviceName, deviceURL string, ctrl *dataTypes.FlowControl) (int, error) {
	nits, err := aiAnalysis.GetIntAIResponse("You're given a phone model, and you need to output how many "+
		"nits at max brightness its display has. If you don't know output 0. Output just a number.", deviceName, ctrl)
	if err != nil || nits == 0 || nits == 123 {
		log.Printf("in helperSpecFunctions.getNitsFromAi (device: %v, url: %v)\nfailed to get response from ai",
			deviceName, deviceURL)
		return 0, err
	}
	return nits, nil
}

func extractNits(deviceName, deviceURL string, details []string) (int, error) {
	for _, detail := range details {
		items := strings.Split(detail, ",")
		for _, item := range items {
			if strings.Contains(item, "nits") {
				if strings.Contains(item, "typ") {
					continue
				}
				nits, err := helpers.ExtractFloat(item)
				if err != nil {
					return 0, err
				}
				return int(nits), nil
			}
		}
	}
	errMsg := fmt.Sprintf("in helperSpecFunctions.extractNits (device: %v, url: %v)\nfailed to find display nits",
		deviceName, deviceURL)
	return 0, errorTypes.NewParsingError(errMsg)
}

func setBatterySize(deviceName, deviceURL string, curSpecs *dataTypes.Specifications, specsByKeys []SpecByKey, ctrl *dataTypes.FlowControl) error {
	for _, detail := range specsByKeys {
		if detail.Key == "Type" {
			batterySize, err := helpers.ExtractFloat(strings.Join(detail.Val, ""))
			if err != nil {
				errMsg := fmt.Sprintf("in helperSpecFunctions.setBatterySize (device: %v, url: %v)\nerror extracting battery size: %v", deviceName, deviceURL, err)
				log.Printf(errMsg)
				parsingErrorLogger.LogErrorInJsonFile(errMsg, ctrl)
				errorMonitoring.IncrementError(errorMonitoring.ParsingError, ctrl)
				return err
			}
			curSpecs.BatteryCapacity = batterySize
			return nil
		}
	}
	errMsg := fmt.Sprintf("in helperSpecFunctions.setBatterySize (device: %v, url: %v)\nfailed to find battery size", deviceName, deviceURL)
	log.Printf(errMsg)
	parsingErrorLogger.LogErrorInJsonFile(errMsg, ctrl)
	errorMonitoring.IncrementError(errorMonitoring.ParsingError, ctrl)
	return errorTypes.NewParsingError(errMsg)
}

func extractDisplaySize(deviceName, deviceURL string, details []string, ctrl *dataTypes.FlowControl) (float64, error) {
	for _, detail := range details {
		if strings.Contains(detail, "inches") {
			size, err := helpers.ExtractFloat(helpers.GetBeforeSubstring(detail, "inches"))
			if err != nil {
				errMsg := "in helperSpecFunctions.extractDisplaySize" +
					fmt.Sprintf("(device: %v, url: %v)\nerror to extract float display size: %v", deviceName,
						deviceURL, err)
				log.Println(errMsg)
				parsingErrorLogger.LogErrorInJsonFile(errMsg, ctrl)
				errorMonitoring.IncrementError(errorMonitoring.ParsingError, ctrl)
				return 0, errorTypes.NewParsingError(errMsg)
			}
			return size, nil
		}
	}
	errMsg := fmt.Sprintf("in helperSpecFunctions.extractDisplaySize (device: %v, url: %v)\nfailed to find display size",
		deviceName, deviceURL)
	log.Println(errMsg)
	parsingErrorLogger.LogErrorInJsonFile(errMsg, ctrl)
	errorMonitoring.IncrementError(errorMonitoring.ParsingError, ctrl)
	return 0, errorTypes.NewParsingError(errMsg)
}

func extractDisplayResolution(deviceName, deviceURL string, details []string, ctrl *dataTypes.FlowControl) (string, error) {
	for _, detail := range details {
		if strings.Contains(detail, " pixels") {
			return strings.TrimSpace(helpers.GetBeforeSubstring(detail, " pixels")), nil
		}
	}
	errMsg := fmt.Sprintf("in helperSpecFunctions.extractDisplayResolution (device: %v, url: %v)\nfailed to parse display resolution",
		deviceName, deviceURL)
	log.Println(errMsg)
	parsingErrorLogger.LogErrorInJsonFile(errMsg, ctrl)
	errorMonitoring.IncrementError(errorMonitoring.ParsingError, ctrl)
	return "", errorTypes.NewParsingError(errMsg)
}

func extractRefreshRate(details []string) int {
	for _, detail := range details {
		if strings.Contains(detail, "Hz") {
			formattedStringSlice := strings.Split(helpers.GetBeforeSubstring(detail, "Hz"), " ")
			refreshRate, err := helpers.ExtractFloat(formattedStringSlice[len(formattedStringSlice)-1])
			if err != nil {
				return 60
			}
			return int(refreshRate)
		}
	}
	return 60
}

func getPixelDensity(deviceName, deviceURL string, displayResolution string, displaySize float64, ctrl *dataTypes.FlowControl) (float64, error) {
	if !strings.Contains(displayResolution, "x") {
		errMsg := fmt.Sprintf("in helperSpecFunctions.getPixelDensity (device: %v, url: %v)\ndisplay resolution does not contain x",
			deviceName, deviceURL)
		log.Println(errMsg)
		parsingErrorLogger.LogErrorInJsonFile(errMsg, ctrl)
		errorMonitoring.IncrementError(errorMonitoring.ParsingError, ctrl)
		return 0, errorTypes.NewParsingError(errMsg)
	}

	seperatedResolutionStrings := strings.Split(displayResolution, " x ")

	var errMsg string
	resolution1, err1 := strconv.Atoi(seperatedResolutionStrings[0])
	if err1 != nil {
		errMsg = "in helperSpecFunctions.getPixelDensity " +
			fmt.Sprintf("in helperSpecFunctions.getPixelDensity (device: %v, url: %v)\n",
				deviceName, deviceURL) + "failed to parse 1st part of resolution"
	}
	resolution2, err2 := strconv.Atoi(seperatedResolutionStrings[1])
	if err2 != nil {
		errMsg = "in helperSpecFunctions.getPixelDensity " +
			fmt.Sprintf("in helperSpecFunctions.getPixelDensity (device: %v, url: %v)\n",
				deviceName, deviceURL) + "failed to parse 2nd part of resolution"
	}
	if err1 != nil || err2 != nil {
		log.Println(errMsg)
		parsingErrorLogger.LogErrorInJsonFile(errMsg, ctrl)
		errorMonitoring.IncrementError(errorMonitoring.ParsingError, ctrl)
		return 0, errorTypes.NewParsingError(errMsg)
	}

	resolution := resolution1 * resolution2

	return float64(resolution) / displaySize, nil
}

func setMainCameraSetup(deviceName, deviceURL string, curSpecs *dataTypes.Specifications, specsByKeys []SpecByKey, ctrl *dataTypes.FlowControl) error {
	cameraSetup, err := extractCameraSetup(deviceName, deviceURL, specsByKeys, ctrl)
	if err != nil {
		log.Printf("in helperSpecFunctions.setMainCameraSetup (device: %v, url: %v)\nfailed to extract camera setup", deviceName, deviceURL)
		return err
	}
	curSpecs.MainCamerasSetup = cameraSetup
	return nil
}

func setSelfieCameraSetup(deviceName, deviceURL string, curSpecs *dataTypes.Specifications, specsByKeys []SpecByKey, ctrl *dataTypes.FlowControl) error {
	cameraSetup, err := extractCameraSetup(deviceName, deviceURL, specsByKeys, ctrl)
	if err != nil {
		log.Printf("in helperSpecFunctions.setSelfieCameraSetup (device: %v, url: %v)\n failed to extract camera setup", deviceName, deviceURL)
		return err
	}
	curSpecs.SelfieCamerasSetup = cameraSetup
	return nil
}
