package priceScraper

import (
	"DeviceRecommendationProject/internal/aiAnalysis"
	"DeviceRecommendationProject/internal/dataTypes"
	"DeviceRecommendationProject/internal/errorMonitoring"
	"DeviceRecommendationProject/internal/errorTypes"
	"DeviceRecommendationProject/internal/helpers"
	"DeviceRecommendationProject/internal/parsingErrorLogger"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"io"
	"log"
	"net/url"
	"os"
	"strings"
)

func SetPrice(device *dataTypes.Device, ctrl *dataTypes.FlowControl) error {
	if ctrl.Ctx.Err() != nil {
		log.Printf("stopping priceScraper.getPriceURL: %v", ctrl.Ctx.Err())
		return ctrl.Ctx.Err()
	}
	priceURL, err := getPriceURL(device.Brand+" "+device.Name, "השוואת+מחירים+טלפונים+סלולריים", ctrl)
	if err != nil {
		var aiInstructionErr errorTypes.FailedAiInstructionError
		if errors.As(err, &aiInstructionErr) {
			priceURL, err = getPriceURL(device.Brand+" "+device.Name, "", ctrl)
			if err != nil {
				log.Printf("in priceScraper.SetPrice (device: %v) failed to get price url: %v", device.Name, err)
				parsingErrorLogger.LogErrorInJsonFile(fmt.Sprintf("in priceScraper.SetPrice (device: %v) failed to get price url: %v", device.Name, err), ctrl)
				return err
			}
		} else {
			parsingErrorLogger.LogErrorInJsonFile(fmt.Sprintf("in priceScraper.SetPrice (device: %v) failed to get price url: %v", device.Name, err), ctrl)
			return err
		}
	}
	document, err := helpers.GetDocumentByURL(priceURL, ctrl)
	if err != nil {
		log.Println("in priceScraper.SetPrice failed to get document by url")
		return err
	}
	price, err := getPriceFromDocument(priceURL, document, ctrl)
	if err != nil {
		log.Println("in priceScraper.SetPrice failed to get price from document")
		return err
	}
	device.RealPrice = price
	return nil
}

func getPriceFromDocument(URL string, document *goquery.Document, ctrl *dataTypes.FlowControl) (int, error) {
	priceString := document.Find("h2.price-value.total").Text()
	if priceString == "" {
		errMsg := fmt.Sprintf("in priceScraper.getPriceFromDocument price not found in link: %v", URL)
		log.Println(errMsg)
		parsingErrorLogger.LogErrorInJsonFile(errMsg, ctrl)
		errorMonitoring.IncrementError(errorMonitoring.ParsingError, ctrl)
		return 0, errorTypes.NewParsingError(errMsg)
	}
	price, err := helpers.ExtractFloat(priceString)
	if err != nil {
		errMsg := fmt.Sprintf("in priceScraper.getPriceFromDocument (url: %v) failed to parse price: %v", URL, err)
		log.Printf(errMsg)
		parsingErrorLogger.LogErrorInJsonFile(errMsg, ctrl)
		errorMonitoring.IncrementError(errorMonitoring.ParsingError, ctrl)
		return 0, err
	}
	return int(price), nil
}

func getPriceURL(brandAndName, searchTerm string, ctrl *dataTypes.FlowControl) (string, error) {
	if ctrl.Ctx.Err() != nil {
		log.Printf("stopping priceScraper.getPriceURL: %v", ctrl.Ctx.Err())
		return "", ctrl.Ctx.Err()
	}

	apiKey := os.Getenv("CUSTOM_SEARCH_KEY")
	searchEngineID := os.Getenv("PRICE_SEARCH_ENGINE_ID")
	brandAndName = strings.ReplaceAll(brandAndName, " ", "+")
	priceUrl := fmt.Sprintf("https://www.googleapis.com/customsearch/v1?key=%s&cx=%s&q=%s",
		apiKey, searchEngineID, url.QueryEscape(brandAndName+searchTerm))

	resp, err := helpers.GetRespByURL(priceUrl, ctrl)
	if err != nil {
		log.Printf("in priceScraper.getPriceURL failed to get response (device: %v)", brandAndName)
		return "", err
	}
	defer func(Body io.ReadCloser) {
		err = Body.Close()
		if err != nil {
			log.Printf("WARNING: Failed to close HTML reader: %v", err)
			errorMonitoring.IncrementError(errorMonitoring.CleanUpError, ctrl)
		}
	}(resp.Body)

	var result struct {
		Items []struct {
			Link    string `json:"link"`
			Title   string `json:"title"`
			Snippet string `json:"snippet"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Println("in priceScraper.getPriceURL failed to decode results")
		parsingErrorLogger.LogErrorInJsonFile(fmt.Sprintf("in priceScraper.getPriceURL failed decode results (device: %v)", brandAndName), ctrl)
		return "", errorTypes.NewParsingError(fmt.Sprintf("in priceScraper.getPriceURL failed decode results (device: %v)", brandAndName))
	}

	for _, item := range result.Items {
		isCorrectUrl, err := aiAnalysis.IsCorrectWebpage("You get a phone model in the format "+
			"\"[brand]+[phone name]\" and a description of a webpage written in hebrew. "+
			"You need to return TRUE if the webpage is solely about the current phone model"+
			", or FALSE otherwise",
			brandAndName, item.Title+" "+item.Snippet, ctrl)
		if err != nil {
			log.Printf("in priceScraper.getPriceURL failed to check if url leads to correct webpage (device: %v)", brandAndName)
			return "", err
		}

		icr, err := isComponentReplacement(item.Link, ctrl)
		if err != nil {
			log.Printf("in priceScraper.getPriceURL failed to check if url leads to component replacement (device: %v)", brandAndName)
			return "", err
		}
		isCorrectUrl = isCorrectUrl && !icr
		if strings.Contains(item.Link, "zap.co.il") && isCorrectUrl {
			log.Printf("in priceScraper.getPriceURL successfully found price url with search term: %v (device: %v)", searchTerm, brandAndName)
			return item.Link, err
		}
	}
	log.Printf("in priceScraper.getPriceURL failed to find price url with search term: %v (device: %v)", searchTerm, brandAndName)
	return "", errorTypes.NewFailedAiInstructionError(fmt.Sprintf("in priceScraper.getPriceURL failed find price url (device: %v)", brandAndName))
}

func SetPriceCategory(device *dataTypes.Device, ctrl *dataTypes.FlowControl) error {
	if ctrl.Ctx.Err() != nil {
		log.Printf("stopping priceScraper.SetPriceCategory: %v", ctrl.Ctx.Err())
		return ctrl.Ctx.Err()
	}

	priceCategory, err := aiAnalysis.GetPriceCategory(device.Name, ctrl)
	if err != nil {
		log.Printf("in priceScraper.setPriceCategory failed to find price category (device: %v)", device.Name)
		return err
	}
	device.PriceCategory = priceCategory
	return nil
}

func isComponentReplacement(url string, ctrl *dataTypes.FlowControl) (bool, error) {
	if ctrl.Ctx.Err() != nil {
		log.Printf("stopping priceScraper.isComponentReplacement: %v", ctrl.Ctx.Err())
		return false, ctrl.Ctx.Err()
	}

	doc, err := helpers.GetDocumentByURL(url, ctrl)
	if err != nil {
		log.Printf("in priceScraper.isComponentReplacement: %v", err)
		return false, err
	}
	selector := `a[href="/models.aspx?sog=e-cellphone"][aria-label="השוואת מחירים טלפונים סלולריים"]`
	componentReplacementString := doc.Find(selector).Text()
	if componentReplacementString != "" {
		return false, nil
	}

	return true, nil
}
