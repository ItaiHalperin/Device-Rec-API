package dataPipelineManager

import (
	"DeviceRecommendationProject/internal/benchmarkScraper"
	"DeviceRecommendationProject/internal/dataAccessLayer"
	"DeviceRecommendationProject/internal/dataTypes"
	"DeviceRecommendationProject/internal/errorTypes"
	"DeviceRecommendationProject/internal/helpers"
	"DeviceRecommendationProject/internal/priceScraper"
	"DeviceRecommendationProject/internal/reviewer"
	"DeviceRecommendationProject/internal/specAPI"
	"context"
	"log"
	"reflect"
	"time"
)

func LaunchDataCollectionProcess(dalForEnqueuer, dalForUploader dataAccessLayer.DataAccessLayer) (<-chan struct{}, func()) {
	ctx, cancel := context.WithCancel(context.Background())
	stopChannel := make(chan struct{}, 1)

	ctrl := dataTypes.FlowControl{
		Ctx:                        ctx,
		StopOnTooManyErrorsChannel: stopChannel,
	}

	go func() {
		log.Println("Launching Enqueuer...")
		launchEnqueuer(dalForEnqueuer, &ctrl)
	}()

	go func() {
		log.Println("Launching Uploader...")
		launchUploader(dalForUploader, &ctrl)
	}()

	return stopChannel, cancel
}

func launchEnqueuer(dal dataAccessLayer.DataAccessLayer, ctrl *dataTypes.FlowControl) {
	for {
		if ctrl.Ctx.Err() != nil {
			log.Printf("stopping dataPiplineManager.launchEnqueuer: %v", ctrl.Ctx.Err())
			return
		}
		namesAndLinks, err := specAPI.GatherAllDeviceNamesAndLinks(ctrl)
		if err != nil {
			time.Sleep(10 * time.Second)
			continue
		}
		if len(namesAndLinks) == 0 {
			time.Sleep(24 * time.Second)
			continue
		}
		err = dal.Database.EnqueueDeviceBatch(namesAndLinks, ctrl)
		if err != nil {
			time.Sleep(10 * time.Second)
			continue
		}

		time.Sleep(5 * time.Minute)
	}
}

func handleError(err error, message string, deviceName string, sleepDuration time.Duration) {
	log.Printf("in dataPipelineManager.launchUploader (device: %s) %s: %v",
		deviceName, message, err)
	time.Sleep(sleepDuration)
}

func processNormalization(dal dataAccessLayer.DataAccessLayer, device *dataTypes.Device,
	ctrl *dataTypes.FlowControl) (dataTypes.MinMaxValues, error) {
	minMaxValues, err := dal.Database.GetValidatedAndUnvalidatedMinMaxValues(ctrl)
	if err != nil {
		return dataTypes.MinMaxValues{}, err
	}

	newMinMax := helpers.GetNewMinMax(device, minMaxValues)
	reviewer.SetNormalizedReviewScore(newMinMax, device)

	if !reflect.DeepEqual(newMinMax, minMaxValues.Validated) {
		return dataTypes.MinMaxValues{}, dal.Database.NormalizeUnvalidatedScores(newMinMax, ctrl)
	}
	return newMinMax, nil
}

func handleBenchmarkEstimation(dal dataAccessLayer.DataAccessLayer, count int, limit int, ctrl *dataTypes.FlowControl) (int, error) {
	if count > limit {
		if err := dal.Database.ReestimateBenchmarks(ctrl); err != nil {
			return count, err
		}
		return 0, nil
	}
	return count, nil
}

func launchUploader(dal dataAccessLayer.DataAccessLayer, ctrl *dataTypes.FlowControl) {
	numberOfEstimatedBenchmarks := 0
	benchmarkCycleLimit := 3

	for {
		if ctrl.Ctx.Err() != nil {
			log.Printf("stopping dataPiplineManager.launchUploader: %v", ctrl.Ctx.Err())
			return
		}

		deviceInQueue, err := dal.Database.Dequeue(ctrl)
		if err != nil {
			if errorTypes.IsMissingDocumentError(err) {
				handleError(err, "no device to dequeue", "", 10*time.Second)
				continue
			}
			handleError(err, "failed to dequeue", "", 10*time.Second)
			continue
		}

		device, err := gatherData(deviceInQueue, dal, ctrl)
		if err != nil {
			handleError(err, "data gathering failed", deviceInQueue.Name, 10*time.Second)
		}

		if device.Benchmark.IsEstimatedBenchmark {
			numberOfEstimatedBenchmarks++
		}

		newMinMax, err := processNormalization(dal, device, ctrl)
		if err != nil {
			handleError(err, "normalization failed", deviceInQueue.Name, 10*time.Second)
			continue
		}
		if err = dal.Database.UploadDevice(device, newMinMax, ctrl); err != nil {
			handleError(err, "failed to upload device", deviceInQueue.Name, 10*time.Second)
			continue
		}

		numberOfEstimatedBenchmarks, err = handleBenchmarkEstimation(dal, numberOfEstimatedBenchmarks,
			benchmarkCycleLimit, ctrl)
		if err != nil {
			handleError(err, "failed to reestimate benchmarks", deviceInQueue.Name, 10*time.Second)
			continue
		}

		time.Sleep(30 * time.Second)
	}
}

func gatherData(deviceInQueue dataTypes.DeviceInQueue, dal dataAccessLayer.DataAccessLayer, ctrl *dataTypes.FlowControl) (*dataTypes.Device, error) {
	device := &dataTypes.Device{}
	device.Image = deviceInQueue.Image
	err := specAPI.SetSpecs(device, deviceInQueue.Detail, ctrl)
	if err != nil {
		log.Printf("in dataPipelineManager.gatherData (device: %v) failed to set specs: %v", deviceInQueue.Name, err)
		return &dataTypes.Device{}, err
	}
	err = priceScraper.SetPrice(device, ctrl)
	if err != nil {
		log.Printf("in dataPipelineManager.gatherData (device: %v) failed to set price: %v", deviceInQueue.Name, err)
		return &dataTypes.Device{}, err
	}
	err = priceScraper.SetPriceCategory(device, ctrl)
	if err != nil {
		log.Printf("in dataPipelineManager.launchUploader (device: %v) failed to set price category: %v", deviceInQueue.Name, err)
		return &dataTypes.Device{}, err
	}
	err = benchmarkScraper.SetBenchmarkScores(device, ctrl)
	if err != nil {
		if errorTypes.IsNoSuchPhoneBenchmarkError(err) {
			log.Printf("in dataPipelineManager.launchUploader (device: %v) failed to find benchmark: %v", deviceInQueue.Name, err)
			device.Benchmark.IsEstimatedBenchmark = true
			err = dal.Database.SetLastYearEquivalentBenchmarkScores(device, ctrl)
			if err != nil {
				log.Printf("in dataPipelineManager.launchUploader (device: %v) failed to set last year equivalent device: %v", deviceInQueue.Name, err)
			}
		} else {
			log.Printf("in dataPipelineManager.launchUploader (device: %v) failed to access benchmark page: %v", deviceInQueue.Name, err)
			return &dataTypes.Device{}, err
		}
	}
	err = reviewer.Review(device, ctrl)
	if err != nil {
		log.Printf("in dataPipelineManager.launchUploader (device: %v) failed to review device: %v", deviceInQueue.Name, err)
		return &dataTypes.Device{}, err
	}
	return device, nil
}
