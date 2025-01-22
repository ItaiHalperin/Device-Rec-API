package specAPI

import (
	"SimpleWeb/internal/dataTypes"
	"SimpleWeb/internal/errorMonitoring"
	"SimpleWeb/internal/errorTypes"
	"SimpleWeb/internal/helpers"
	"SimpleWeb/internal/parsingErrorLogger"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"
)

const (
	maxApplePage   = 4
	maxGooglePage  = 1
	maxSamsungPage = 9
)

type SpecByKey struct {
	Key string   `json:"key"`
	Val []string `json:"val"`
}

type SpecsByTitle struct {
	Title       string      `json:"title"`
	SpecsByKeys []SpecByKey `json:"specs"`
}

type DeviceData struct {
	Brand       string         `json:"brand"`
	PhoneName   string         `json:"phone_name"`
	ReleaseDate string         `json:"release_date"`
	RawSpecs    []SpecsByTitle `json:"specifications"`
}

type APIResponse struct {
	Status bool       `json:"status"`
	Data   DeviceData `json:"data"`
}

func getMaxPage(brand string) int {
	switch brand {
	case "Apple":
		return maxApplePage
	case "Google":
		return maxGooglePage
	case "Samsung":
		return maxSamsungPage
	default:
		return -1
	}
}

func SetSpecs(device *dataTypes.Device, url string, ctrl *dataTypes.FlowControl) error {
	resp, err := helpers.GetRespByURL(url, ctrl)
	if err != nil {
		log.Println("in specAPI.SetSpecs error getting response")
		return err
	}
	defer func(Body io.ReadCloser) {
		err = Body.Close()
		if err != nil {
			log.Printf("WARNING: Failed to close HTML reader: %v", err)
			errorMonitoring.IncrementError(errorMonitoring.CleanUpError, ctrl)
		}
	}(resp.Body)

	var responseData APIResponse
	if err = json.NewDecoder(resp.Body).Decode(&responseData); err != nil {
		log.Printf("in specAPI.SetSpecs failed to decode json: %v", err)
		parsingErrorLogger.LogErrorInJsonFile(fmt.Sprintf("in specAPI.SetSpecs failed to decode json: %v", err), ctrl)
		errorMonitoring.IncrementError(errorMonitoring.ParsingError, ctrl)
		return errorTypes.NewParsingError("in specAPI.SetSpecs failed to decode search results")
	}

	device.Name = strings.TrimSpace(responseData.Data.PhoneName)
	device.Brand = strings.TrimSpace(responseData.Data.Brand)

	curSpecs := dataTypes.Specifications{}
	err = setReleaseDate(device.Name, url, &curSpecs, responseData.Data.ReleaseDate, ctrl)
	if err != nil {
		log.Printf("in specAPI.SetSpecs (device: %v, url: %v)\n error setting release date: %v", device.Name, url, err)
		return err
	}
	numOfSpecsCollected := 0
	numOfSpecsExpected := 4
	for _, spec := range responseData.Data.RawSpecs {
		switch spec.Title {
		case "Main Camera":
			err = setMainCameraSetup(device.Name, url, &curSpecs, spec.SpecsByKeys, ctrl)
			if err != nil {
				log.Printf("in specAPI.SetSpecs failed to set main camera setup for device: %v", device.Name)
				return err
			}
			numOfSpecsCollected++
		case "Selfie camera":
			err = setSelfieCameraSetup(device.Name, url, &curSpecs, spec.SpecsByKeys, ctrl)
			if err != nil {
				log.Printf("in specAPI.SetSpecs failed to set selfie camera setup for device: %v", device.Name)
				return err
			}
			numOfSpecsCollected++
		case "Battery":
			err = setBatterySize(device.Name, url, &curSpecs, spec.SpecsByKeys, ctrl)
			if err != nil {
				log.Printf("in specAPI.SetSpecs failed to set battery size for device: %v", device.Name)
				return err
			}
			numOfSpecsCollected++
		case "Display":
			err = setDisplayDetails(device.Name, url, &curSpecs, spec.SpecsByKeys, ctrl)
			if err != nil {
				log.Printf("in specAPI.SetSpecs failed to set display details for device: %v", device.Name)
				return err
			}
			numOfSpecsCollected++
		}

	}

	if numOfSpecsCollected != numOfSpecsExpected {
		errMsg := fmt.Sprintf("in specAPI.SetSpecs (device: %v, url: %v)\nfailed to gather all specs", device.Name, url)
		parsingErrorLogger.LogErrorInJsonFile(errMsg, ctrl)
		return errorTypes.NewParsingError(errMsg)
	}

	pixelDensity, err := getPixelDensity(device.Name, url, curSpecs.DisplayResolution, curSpecs.DisplaySize, ctrl)
	if err != nil {
		errMsg := fmt.Sprintf("in specAPI.SetSpecs (device: %v, url: %v)\nfailed to get the pixel density", device.Name, url)
		log.Printf(errMsg)
		return err
	}
	curSpecs.PixelDensity = pixelDensity
	device.Specs = curSpecs
	return nil
}

func GatherAllDeviceNamesAndLinks(ctrl *dataTypes.FlowControl) (map[string][]string, error) {
	iphones, err := getAllNamesAndLinksByBrand("apple-phones-48", "iphone", "Apple", ctrl, "ipad", "cdma", "watch")
	if err != nil {
		log.Printf("in specAPI.GatherAllDeviceNamesAndLinks failed to get iphones: %v", err)
		return nil, err
	}
	pixels, err := getAllNamesAndLinksByBrand("google-phones-107", "pixel", "Google", ctrl, "tablet", "fold", "watch")
	if err != nil {
		log.Printf("in specAPI.GatherAllDeviceNamesAndLinks failed to get pixels: %v", err)
		return nil, err
	}

	samsungs, err := getAllNamesAndLinksByBrand("samsung-phones-9", "galaxy", "Samsung", ctrl,
		"watch", "tab", "flip", "fold", "(india)", "Grand", "Indulge", "Nexus", "LTE", "Prevail", "Attain",
		" Star ", " zoom ", " Duos ", "(", " Pop ", " S ", " Young ", " Express ", "Core", "alpha", "Sport", "Edge", "ii",
		" Active ", " Quantum ", " Lite ", " Stellar ", " Apollo ", " Ace ", "View", " Light ", "Xcover", "Galaxy M", "Neo")
	if err != nil {
		log.Println("in specAPI.GatherAllDeviceNamesAndLinks failed to get samsungs")
		return nil, err
	}

	var phoneAndLinkMap = make(map[string][]string)
	for _, series := range [][]Phone{iphones, pixels, samsungs} {
		for _, phone := range series {
			phoneAndLinkMap[phone.PhoneName] = []string{phone.Detail, phone.Image}
		}
	}

	return phoneAndLinkMap, nil
}

type Phone struct {
	PhoneName string `json:"phone_name"`
	Detail    string `json:"detail"`
	Image     string `json:"image"`
}

type Response struct {
	Status bool `json:"status"`
	Data   struct {
		Phones []Phone `json:"phones"`
	} `json:"data"`
}

func getAllNamesAndLinksByBrand(brandDirectory, series, brand string, ctrl *dataTypes.FlowControl, excludedTerms ...string) ([]Phone, error) {
	baseURL := "https://phone-specs-api.vercel.app/brands/" + brandDirectory + "?page="
	page := 1
	var allPhones []Phone

	for i := 1; i < getMaxPage(brand); page++ {
		url := fmt.Sprintf("%s%d", baseURL, page)
		log.Printf("Fetching data from: %s\n", url)

		resp, err := helpers.GetRespByURL(url, ctrl)
		if err != nil {
			errMsg := fmt.Sprintf("in specAPI.getAllNamesAndLinksByBrand (url: %v)\nfailed to get response", url)
			log.Println(errMsg)
			return nil, err
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			errMsg := fmt.Sprintf("in specAPI.getAllNamesAndLinksByBrand (url: %v)\nfailed to read resp body", url)
			log.Println(errMsg)
			return nil, err
		}

		var apiResponse Response
		err = json.Unmarshal(body, &apiResponse)
		if err != nil {
			errMsg := fmt.Sprintf("in specAPI.getAllNamesAndLinksByBrand (url: %v)\nfailed to unmarshal resp body", url)
			log.Println(errMsg)
			parsingErrorLogger.LogErrorInJsonFile(errMsg, ctrl)
			return nil, err
		}

		if len(apiResponse.Data.Phones) == 0 {
			log.Printf("in specAPI.getAllNamesAndLinksByBrand finished reading all devices in %v", brandDirectory)
			break
		}

		for _, phone := range apiResponse.Data.Phones {
			if helpers.DoesNameContainExcludedKeywords(phone.PhoneName, excludedTerms) {
				continue
			} else if strings.Contains(strings.ToLower(phone.PhoneName), series) {
				allPhones = append(allPhones, phone)
			}
		}
		err = resp.Body.Close()
		if err != nil {
			errorMonitoring.IncrementError(errorMonitoring.CleanUpError, ctrl)
			return nil, errorTypes.NewParsingError("failed to close response body")
		}
		page++
	}
	return allPhones, nil
}
