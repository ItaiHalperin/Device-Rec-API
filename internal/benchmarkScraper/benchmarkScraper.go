package benchmarkScraper

import (
	"SimpleWeb/internal/dataTypes"
	"SimpleWeb/internal/errorMonitoring"
	"SimpleWeb/internal/errorTypes"
	"SimpleWeb/internal/helpers"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"log"
	"strconv"
	"strings"
)

func SetBenchmarkScores(device *dataTypes.Device, ctrl *dataTypes.FlowControl) error {
	if ctrl.Ctx.Err() != nil {
		log.Printf("stopping benchmarkScraper.SetBenchmarkScores: %v", ctrl.Ctx.Err())
		return ctrl.Ctx.Err()
	}

	singleCoreScore, multiCoreScore, err := GetSingleMultiScores(device.Brand, device.Name, ctrl)
	if err != nil {
		log.Printf("in benchmarkScraper.SetBenchmarkScores failed to get single and multi core scores: %v", err)
		return err
	}

	device.Benchmark.MultiCoreScore = float64(multiCoreScore)
	device.Benchmark.SingleCoreScore = float64(singleCoreScore)
	return nil
}

func GetSingleMultiScores(brand, model string, ctrl *dataTypes.FlowControl) (int, int, error) {
	if ctrl.Ctx.Err() != nil {
		log.Printf("stopping benchmarkScraper.getSingleMultiScores: %v", ctrl.Ctx.Err())
		return 0, 0, ctrl.Ctx.Err()
	}

	var url string
	if brand == "Apple" {
		url = "https://browser.geekbench.com/ios-benchmarks/"
	} else {
		model = brand + " " + model
		url = "https://browser.geekbench.com/android-benchmarks/"
	}

	doc, err := helpers.GetDocumentByURL(url, ctrl)
	if err != nil {
		log.Printf("in benchmarkScraper.getSingleMultiScores failed to get benchmark page: %v", err)
		return 0, 0, err
	}

	singleCoreScore, err := getSingleScore(model, doc, ctrl)
	if err != nil {
		log.Printf("in benchmarkScraper.getSingleMultiScores failed to get single core score: %v", err)
		return 0, 0, err
	}

	multiCoreScore, err := getMultiScore(model, doc, ctrl)
	if err != nil {
		log.Printf("in benchmarkScraper.getSingleMultiScores failed to get multi core score: %v", err)
		return 0, 0, err
	}

	return singleCoreScore, multiCoreScore, nil
}

func getSingleScore(modelName string, doc *goquery.Document, ctrl *dataTypes.FlowControl) (int, error) {
	if ctrl.Ctx.Err() != nil {
		log.Printf("stopping benchmarkScraper.getSingleScore: %v", ctrl.Ctx.Err())
		return 0, ctrl.Ctx.Err()
	}
	return getScore(modelName, "div#single-core.tab-pane.fade.show.active", doc, ctrl)
}

func getMultiScore(modelName string, doc *goquery.Document, ctrl *dataTypes.FlowControl) (int, error) {
	if ctrl.Ctx.Err() != nil {
		log.Printf("stopping benchmarkScraper.getMultiScore: %v", ctrl.Ctx.Err())
		return 0, ctrl.Ctx.Err()
	}
	return getScore(modelName, "div#multi-core.tab-pane.fade", doc, ctrl)
}

func getScore(modelName, selector string, doc *goquery.Document, ctrl *dataTypes.FlowControl) (int, error) {
	if ctrl.Ctx.Err() != nil {
		log.Printf("stopping benchmarkScraper.getScore: %v", ctrl.Ctx.Err())
		return 0, ctrl.Ctx.Err()
	}

	var score int
	var err error

	doc.Find(selector).Find("tr").Each(func(i int, s *goquery.Selection) {
		deviceName := strings.TrimSpace(s.Find("td.name a").Text())
		curScore := strings.TrimSpace(s.Find("td.score").Text())

		if deviceName != "" && curScore != "" {
			if strings.ToLower(deviceName) == strings.ToLower(modelName) {
				score, err = strconv.Atoi(curScore)
				return
			}
		}
	})

	if err != nil {
		errorMonitoring.IncrementError(errorMonitoring.ParsingError, ctrl)
		log.Printf("in benchmarkScraper.getSingleMultiScore, found device %v but error parsing score: %v", modelName, err)
		return 0, errorTypes.NewParsingError(fmt.Sprintf("in benchmarkScraper.getSingleMultiScore found device but error parsing score: %v", err))
	}

	if score == 0 {
		log.Printf("in benchmarkScraper.getSingleMultiScore failed to find device %v in benchmark page", modelName)
		return 0, errorTypes.NewNoSuchPhoneBenchmarkError(fmt.Sprintf("in benchmarkScraper.getSingleMultiScore failed to find device %v in benchmark page", modelName))
	}

	return score, nil
}
