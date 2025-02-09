package reviewer

import (
	"DeviceRecommendationProject/internal/aiAnalysis"
	"DeviceRecommendationProject/internal/dataTypes"
	"DeviceRecommendationProject/internal/errorMonitoring"
	"DeviceRecommendationProject/internal/errorTypes"
	"DeviceRecommendationProject/internal/helpers"
	"DeviceRecommendationProject/internal/parsingErrorLogger"
	"encoding/json"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"log"
	"os"
	"strconv"
	"strings"
)

func Review(device *dataTypes.Device, ctrl *dataTypes.FlowControl) error {
	if ctrl.Ctx.Err() != nil {
		log.Printf("stopping reviewer.Review: %v", ctrl.Ctx.Err())
		return ctrl.Ctx.Err()
	}
	var cnetReviewer Cnet
	var tomsGuideReviewer TomsGuide
	cnetReviewSentiment, cnetReviewMagnitude, err := getSentimentMagnitude(cnetReviewer, device.Name, ctrl)
	if err != nil {
		log.Printf("in reviewer.Review (device: %v) failed to find cnet review: %v", device.Name, err)
		return err
	}
	tomsGuideReviewSentiment, tomsGuideReviewMagnitude, err := getSentimentMagnitude(tomsGuideReviewer, device.Name, ctrl)
	if err != nil {
		log.Printf("in reviewer.Review (device: %v) failed to find tom's guide review: %v", device.Name, err)
		return err
	}
	averageSentiment := (tomsGuideReviewSentiment + cnetReviewSentiment) / 2
	averageMagnitude := (tomsGuideReviewMagnitude + cnetReviewMagnitude) / 2

	device.Review.ReviewMagnitude = averageMagnitude
	device.Review.ReviewSentiment = averageSentiment
	return nil
}

// We get score (-1 to 1), magnitude (0 to infinity), publish time and error
func getSentimentMagnitude(reviewer Reviewer, model string, ctrl *dataTypes.FlowControl) (float64, float64, error) {
	if ctrl.Ctx.Err() != nil {
		log.Printf("stopping reviewer.getSentimentMagnitude: %v", ctrl.Ctx.Err())
		return 0, 0, ctrl.Ctx.Err()
	}

	url, err := GetReviewURLByModel(model, reviewer.GetDomain(), ctrl)
	if err != nil {
		log.Printf("in reviewer.getSentimentMagnitude (device: %v) failed to get review url: %v", model, err)
		return 0, 0, err
	}

	doc, err := helpers.GetDocumentByURL(url, ctrl)
	if err != nil {
		log.Printf("in reviewer.getSentimentMagnitude (device: %v) failed to get document from review url: %v", model, err)
		return 0, 0, err
	}

	review, err := reviewer.getReviewString(model, url, doc, ctrl)
	if err != nil {
		log.Printf("in reviewer.getSentimentMagnitude (device: %v) failed to get review string from document: %v", model, err)
		return 0, 0, err
	}
	sentiment, magnitude, err := aiAnalysis.AnalyseSentimentMagnitude(review, ctrl)
	if err != nil {
		log.Printf("in reviewer.getSentimentMagnitude (device: %v) failed to analyze review string: %v", model, err)
		return 0, 0, err
	}

	stars, err := reviewer.GetStars(model, doc, ctrl)
	if err != nil {
		return sentiment, magnitude, nil
	}

	return sentiment*0.6 + ((stars-3)/2)*0.4, magnitude, nil
}

func SetNormalizedReviewScore(newMinMaxMagnitudeSentiment dataTypes.MinMaxValues, device *dataTypes.Device) {
	normalizedMagnitude := helpers.CalculateNormalizedValue(newMinMaxMagnitudeSentiment.Magnitude.Min, newMinMaxMagnitudeSentiment.Magnitude.Max, device.Review.ReviewMagnitude)
	normalizedSentiment := helpers.CalculateNormalizedValue(newMinMaxMagnitudeSentiment.Sentiment.Min, newMinMaxMagnitudeSentiment.Sentiment.Max, device.Review.ReviewSentiment)

	reviewScore := normalizedSentiment * normalizedMagnitude
	device.Review.UnvalidatedReviewScore = reviewScore
}

func GetReviewURLByModel(brandAndName string, reviewerDomain string, ctrl *dataTypes.FlowControl) (string, error) {
	if ctrl.Ctx.Err() != nil {
		log.Printf("stopping reviewer.GetReviewURLByModel: %v", ctrl.Ctx.Err())
		return "", ctrl.Ctx.Err()
	}

	apiKey := os.Getenv("CUSTOM_SEARCH_KEY")
	searchEngineID := os.Getenv("REVIEW_SEARCH_ENGINE_ID")
	searchTerm := strings.ReplaceAll(brandAndName, " ", "+") + "+review+" + reviewerDomain

	url := fmt.Sprintf("https://www.googleapis.com/customsearch/v1?key=%s&cx=%s&q=%s",
		apiKey, searchEngineID, searchTerm)

	resp, err := helpers.GetRespByURL(url, ctrl)
	if err != nil {
		log.Printf("in reviewer.GetReviewURLByModel (device: %v) failed to get review search results: %v", brandAndName, err)
		return "", err
	}

	var result struct {
		Items []struct {
			Link    string `json:"link"`
			Title   string `json:"title"`
			Snippet string `json:"snippet"`
		} `json:"items"`
	}
	if err = json.NewDecoder(resp.Body).Decode(&result); err != nil {
		log.Printf("in reviewer.GetReviewURLByModel (device: %v) failed to decode search results: %v", brandAndName, err)
		errorMonitoring.IncrementError(errorMonitoring.ParsingError, ctrl)
		parsingErrorLogger.LogErrorInJsonFile(fmt.Sprintf("in reviewer.GetReviewURLByModel (device: %v) failed to decode search results in search", brandAndName), ctrl)
		return "", errorTypes.NewParsingError(fmt.Sprintf("in reviewer.GetReviewURLByModel (device: %v) failed to decode search results", brandAndName))
	}

	for _, item := range result.Items {
		isCorrectUrl, err := aiAnalysis.IsCorrectWebpage("You get a phone model in the format "+
			"\"[brand]+[phone name]\" and a description of a review. "+
			"You need to return TRUE if the review is about the current phone model, "+
			"or FALSE otherwise. Ignore suffixes like 5G, focus on model name and number",
			brandAndName, item.Title+" "+item.Snippet, ctrl)
		if err != nil {
			log.Printf("in reviewer.GetReviewURLByModel (device: %v) failed to check if url leads to correct webpage: %v", brandAndName, err)
			return "", err
		}
		if strings.Contains(item.Link, reviewerDomain) && isCorrectUrl {
			return item.Link, err
		}
	}
	errMsg := fmt.Sprintf("in reviewer.GetReviewURLByModel (device: %v) failed to get review url with ai",
		brandAndName)
	log.Printf(errMsg)
	parsingErrorLogger.LogErrorInJsonFile(errMsg, ctrl)
	return "", errorTypes.NewFailedAiInstructionError(errMsg)
}

type Reviewer interface {
	GetStars(string, *goquery.Document, *dataTypes.FlowControl) (float64, error)
	getReviewString(string, string, *goquery.Document, *dataTypes.FlowControl) (string, error)
	GetDomain() string
}

type TomsGuide struct{}

func (t TomsGuide) GetDomain() string {
	return "tomsguide.com"
}

func (t TomsGuide) GetStars(model string, document *goquery.Document, ctrl *dataTypes.FlowControl) (float64, error) {
	if ctrl.Ctx.Err() != nil {
		log.Printf("stopping reviewer.GetStars: %v", ctrl.Ctx.Err())
		return 0, ctrl.Ctx.Err()
	}

	span := document.Find("span.chunk.rating")

	// Get the "aria-label" attribute from the span element
	ariaLabel, exists := span.Attr("aria-label")
	if !exists || len(ariaLabel) < 9 {
		log.Printf("in reviewer.GetStars (tom's guide, device: %v) failed to find stars", model)
		return 0, errorTypes.NewParsingError(fmt.Sprintf("in reviewer.GetStars (tom's guide, device: %v) failed to find stars", model))
	}

	ratingStr := string(ariaLabel[8])
	rating, err := strconv.ParseFloat(ratingStr, 64)
	if err != nil {
		log.Printf("in reviewer.getStars (tom's guide, device: %v) failed to convert rating to float: %v", model, err)
		return 0, errorTypes.NewParsingError(fmt.Sprintf("in reviewer.getStars (tom's guide, device: %v) failed to convert rating to float: %v", model, err))
	}

	return rating, nil
}

func (t TomsGuide) getReviewString(model, url string, document *goquery.Document, ctrl *dataTypes.FlowControl) (string, error) {
	if ctrl.Ctx.Err() != nil {
		log.Printf("stopping reviewer.getReviewString: %v", ctrl.Ctx.Err())
		return "", ctrl.Ctx.Err()
	}

	var paragraphs []string
	document.Find("p").Each(func(i int, s *goquery.Selection) {
		paragraphText := strings.TrimSpace(s.Text())
		if paragraphText != "" {
			paragraphs = append(paragraphs, paragraphText)
		}
	})
	if len(paragraphs) == 0 {
		errMsg := fmt.Sprintf("in reviewer.GetReviewerString (tom's guide, device: %v, url: %v)\nfailed to get review text", model, url)
		log.Printf(errMsg)
		errorMonitoring.IncrementError(errorMonitoring.ParsingError, ctrl)
		parsingErrorLogger.LogErrorInJsonFile(errMsg, ctrl)
		return "", errorTypes.NewParsingError(errMsg)
	}
	return strings.Join(paragraphs, " "), nil
}

type Cnet struct{}

func (c Cnet) GetDomain() string {
	return "cnet.com"
}

func (c Cnet) GetStars(model string, document *goquery.Document, ctrl *dataTypes.FlowControl) (float64, error) {
	if ctrl.Ctx.Err() != nil {
		log.Printf("stopping reviewer.GetStars (cnet): %v", ctrl.Ctx.Err())
		return 0, ctrl.Ctx.Err()
	}

	starsString, err := c.tryFirstStarsFormat(model, document, ctrl)
	if err == nil {
		starsString = starsString[:3]
		stars, err := strconv.ParseFloat(starsString, 64)
		if err == nil {
			return stars / 2, nil
		}
	}
	log.Printf("in reviewer.GetStars (cnet, device: %v) failed to find first stars format, trying second...", model)

	starsString, err = c.trySecondStarsFormat(model, document, ctrl)
	if err == nil {
		starsString = starsString[:3]
		stars, err := strconv.ParseFloat(starsString, 64)
		if err == nil {
			return stars / 2, nil
		}
	}
	log.Printf("in reviewer.GetStars (cnet, device: %v) failed to find second stars format, trying third...", model)

	starsString, err = c.tryThirdStarsFormat(model, document, ctrl)
	if err != nil {
		log.Printf("in reviewer.GetStars (cnet, device: %v) failed to find third (and final) stars format", model)
		return 0, errorTypes.NewParsingError(fmt.Sprintf("in reviewer.GetStars (cnet, device: %v) failed to find third (and final) stars format", model))
	}

	starsString = starsString[:3]
	stars, err := strconv.ParseFloat(starsString, 64)
	if err != nil {
		log.Printf("in reviewer.GetStars (cnet, device: %v) failed to find third (and final) stars format", model)
		return 0, errorTypes.NewParsingError(fmt.Sprintf("in reviewer.GetStars (cnet, device: %v) failed to find third (and final) stars format", model))
	}

	return stars / 2, nil
}

func (c Cnet) getReviewString(model, url string, document *goquery.Document, ctrl *dataTypes.FlowControl) (string, error) {
	if ctrl.Ctx.Err() != nil {
		log.Printf("stopping reviewer.getReviewString: %v", ctrl.Ctx.Err())
		return "", ctrl.Ctx.Err()
	}
	var paragraphs []string
	document.Find("p").Each(func(i int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		if text != "" {
			paragraphs = append(paragraphs, text)
		}
	})
	if len(paragraphs) == 0 {
		errMsg := fmt.Sprintf("in reviewer.GetReviewerString (cnet, device: %v, url: %v)\nfailed to get review text", model, url)
		log.Printf(errMsg)
		errorMonitoring.IncrementError(errorMonitoring.ParsingError, ctrl)
		parsingErrorLogger.LogErrorInJsonFile(errMsg, ctrl)
		return "", errorTypes.NewParsingError(errMsg)
	}
	return strings.Join(paragraphs, " "), nil
}

func (c Cnet) tryFirstStarsFormat(model string, document *goquery.Document, ctrl *dataTypes.FlowControl) (string, error) {
	if ctrl.Ctx.Err() != nil {
		log.Printf("stopping reviewer.tryFirstStarsFormat: %v", ctrl.Ctx.Err())
		return "", ctrl.Ctx.Err()
	}
	starsString := document.Find(`div[data-cy="reviewRating"].c-shortcodeReviewRedesign_rating.g-text-bold`).Text()
	starsString = strings.TrimSpace(starsString)
	starsRunes := []rune(starsString)
	if len(starsRunes) < 3 {
		return "", errorTypes.NewParsingError(fmt.Sprintf("in Cnet.tryFirstStarsFormat (cnet, device: %v) stars too short", model))
	}

	return string(starsRunes), nil
}

func (c Cnet) trySecondStarsFormat(model string, document *goquery.Document, ctrl *dataTypes.FlowControl) (string, error) {
	if ctrl.Ctx.Err() != nil {
		log.Printf("stopping reviewer.trySecondStarsFormat: %v", ctrl.Ctx.Err())
		return "", ctrl.Ctx.Err()
	}
	starsString := document.Find("div.c-reviewCard_data-score.g-text-bold").Text()
	starsString = strings.TrimSpace(starsString)
	starsRunes := []rune(starsString)
	if len(starsRunes) < 3 {
		return "", errorTypes.NewParsingError(fmt.Sprintf("in Cnet.trySecondStarsFormat (cnet, device: %v) stars too short", model))
	}

	return string(starsRunes), nil
}

func (c Cnet) tryThirdStarsFormat(model string, document *goquery.Document, ctrl *dataTypes.FlowControl) (string, error) {
	if ctrl.Ctx.Err() != nil {
		log.Printf("stopping reviewer.tryThirdStarsFormat: %v", ctrl.Ctx.Err())
		return "", ctrl.Ctx.Err()
	}
	starsString := document.Find(`div[data-cy="reviewRating"].c-shortcodeReview_rating.g-text-bold`).Text()
	starsString = strings.TrimSpace(starsString)
	starsRunes := []rune(starsString)
	if len(starsRunes) < 3 {
		return "", errorTypes.NewParsingError(fmt.Sprintf("in Cnet.tryThirdStarsFormat (cnet, device: %v) stars too short", model))
	}

	return string(starsRunes), nil
}
