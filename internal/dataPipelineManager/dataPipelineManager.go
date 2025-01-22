package dataPipelineManager

import (
	"SimpleWeb/internal/benchmarkScraper"
	"SimpleWeb/internal/dataAccessLayer"
	"SimpleWeb/internal/dataTypes"
	"SimpleWeb/internal/errorTypes"
	"SimpleWeb/internal/priceScraper"
	"SimpleWeb/internal/reviewer"
	"SimpleWeb/internal/specAPI"
	"context"
	"log"
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

func launchUploader(dal dataAccessLayer.DataAccessLayer, ctrl *dataTypes.FlowControl) {
	numberOfEstimatedBenchmarks := 0
	benchmarkCycleLimit := 1
	for {
		if ctrl.Ctx.Err() != nil {
			log.Printf("stopping dataPiplineManager.launchUploader: %v", ctrl.Ctx.Err())
			return
		}
		minMaxValues, err := dal.Database.GetValidatedAndUnvalidatedMinMaxValues(ctrl)
		if err != nil {
			log.Fatalf("error getting validated and unvalid dataPiplineManager.launchUploader: %v", err)
			return
		}
		err = dal.Database.NormalizeUnvalidatedScores(minMaxValues.Validated, ctrl)
		if err != nil {
			log.Fatalf("error normalizing unvalid dataPiplineManager.launchUploader: %v", err)
			return
		}
		deviceInQueue, err := dal.Database.Dequeue(ctrl)
		if err != nil {
			if errorTypes.IsMissingDocumentError(err) {
				log.Printf("in dataPipelineManager.launchUploader no device to dequeue: %v", err)
				time.Sleep(30 * time.Second)
				continue
			} else {
				log.Printf("in dataPipelineManager.launchUploader failed to dequeue: %v", err)
				time.Sleep(10 * time.Second)
				continue
			}
		}
		device := &dataTypes.Device{}
		device.Image = deviceInQueue.Image
		err = specAPI.SetSpecs(device, deviceInQueue.Detail, ctrl)
		if err != nil {
			log.Printf("in dataPipelineManager.launchUploader (device: %v) failed to set specs: %v", deviceInQueue.Name, err)
			time.Sleep(10 * time.Second)
			continue
		}
		err = priceScraper.SetPrice(device, ctrl)
		if err != nil {
			log.Printf("in dataPipelineManager.launchUploader (device: %v) failed to set price: %v", deviceInQueue.Name, err)
			time.Sleep(10 * time.Second)
			continue
		}
		err = priceScraper.SetPriceCategory(device, ctrl)
		if err != nil {
			log.Printf("in dataPipelineManager.launchUploader (device: %v) failed to set price category: %v", deviceInQueue.Name, err)
			time.Sleep(10 * time.Second)
			continue
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
				numberOfEstimatedBenchmarks++
			} else {
				log.Printf("in dataPipelineManager.launchUploader (device: %v) failed to access benchmark page: %v", deviceInQueue.Name, err)
			}
		}
		newMinMax, err := reviewer.Review(device, dal, ctrl)
		if err != nil {
			log.Printf("in dataPipelineManager.launchUploader (device: %v) failed to review device: %v", deviceInQueue.Name, err)
			time.Sleep(10 * time.Second)
			continue
		}
		err = dal.Database.UploadDevice(device, newMinMax, ctrl)
		if err != nil {
			log.Printf("in dataPipelineManager.launchUploader (device: %v) failed to upload device: %v", deviceInQueue.Name, err)
			time.Sleep(10 * time.Second)
			continue
		}
		if numberOfEstimatedBenchmarks > benchmarkCycleLimit {
			err = dal.Database.ReestimateBenchmarks(ctrl)
			if err != nil {
				log.Printf("in dataPipelineManager.launchUploader (device: %v) failed to reestimate benchmarks: %v", deviceInQueue.Name, err)
				time.Sleep(10 * time.Second)
				continue
			} else {
				numberOfEstimatedBenchmarks = 0
			}
		}
		time.Sleep(1 * time.Second)
	}
}
