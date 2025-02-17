package mongoDatabase

import (
	"Device-Rec-API/internal/aiAnalysis"
	"Device-Rec-API/internal/dataTypes"
	"Device-Rec-API/internal/errorTypes"
	"Device-Rec-API/internal/helpers"
	"context"
	"errors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"log"
	"math"
	"time"
)

const (
	yearOverYearIncrease = 0.10
)

func (mdb *MongoDatabase) ReestimateBenchmarks(ctrl *dataTypes.FlowControl) error {
	if ctrl.Ctx.Err() != nil {
		log.Printf("stopping mongoDatabase.ReestimateBenchmarks: %v", ctrl.Ctx.Err())
		return ctrl.Ctx.Err()
	}
	coll := mdb.client.Database(Database).Collection(DeviceDataCollection)

	allDevicesWithEstimatedBenchmark, err := mdb.getAllDevicesWithEstimatedBenchmark(coll, ctrl)
	if err != nil {
		log.Printf("in mongoDatabase.ReestimateBenchmarks failed to get all devices with estimated benchmark: %v", err)
		return err
	}

	helpers.SortDevicesByDate(allDevicesWithEstimatedBenchmark)

	for _, device := range allDevicesWithEstimatedBenchmark {
		err = mdb.SetLastYearEquivalentBenchmarkScores(&device, ctrl)
		if err != nil {
			log.Printf("in mongoDatabase.ReestimateBenchmarks (device: %v) failed to set last year equivalent benchmark score: %v", device.Name, err)
			return err
		}

		update := bson.D{{"$set", bson.D{{"benchmark", device.Benchmark}}}}
		ctx, cancel := context.WithTimeout(ctrl.Ctx, time.Second*30)
		_, err = coll.UpdateByID(ctx, device.ID, update)
		if err != nil {
			log.Printf("in mongoDatabase.ReestimateBenchmarks failed to update device: %v", device.Name)
			cancel()
			return handleMongoError(err, true, ctrl)
		}
		cancel()
	}

	return nil
}

func (mdb *MongoDatabase) getAllDevicesWithEstimatedBenchmark(coll *mongo.Collection, ctrl *dataTypes.FlowControl) ([]dataTypes.Device, error) {
	if ctrl.Ctx.Err() != nil {
		log.Printf("stopping mongoDatabase.getAllDevicesWithEstimatedBenchmark: %v", ctrl.Ctx.Err())
		return nil, ctrl.Ctx.Err()
	}

	allDevices, err := mdb.getAllDevices(coll, ctrl)
	if err != nil {
		log.Println("in mongoDatabase.getAllDevicesWithEstimatedBenchmark failed to get all devices")
		return nil, err
	}

	var devicesWithEstimatedBenchmark []dataTypes.Device
	for _, device := range allDevices {
		if device.Benchmark.IsEstimatedBenchmark {
			devicesWithEstimatedBenchmark = append(devicesWithEstimatedBenchmark, device)
		}
	}

	return devicesWithEstimatedBenchmark, nil
}

func (mdb *MongoDatabase) GetLastYearEquivalentBenchmarkScores(device *dataTypes.Device, ctrl *dataTypes.FlowControl) (float64, float64, error) {
	if ctrl.Ctx.Err() != nil {
		log.Printf("stopping mongoDatabase.GetLastYearEquivalentBenchmarkScores: %v", ctrl.Ctx.Err())
		return 0, 0, ctrl.Ctx.Err()
	}

	coll := mdb.client.Database(Database).Collection(DeviceDataCollection)
	lastYearModelName, err := helpers.DecrementNumberInString(device.Name)
	if err == nil {
		filter := bson.D{{"name", lastYearModelName}}
		var lastYearDevice dataTypes.Device
		ctx, cancel := context.WithTimeout(ctrl.Ctx, time.Second*30)
		defer cancel()
		err := coll.FindOne(ctx, filter).Decode(&lastYearDevice)
		if err == nil {
			return lastYearDevice.Benchmark.SingleCoreScore * (1 + yearOverYearIncrease), lastYearDevice.Benchmark.MultiCoreScore * (1 + yearOverYearIncrease), nil
		}
	}

	lastYearNumber := device.Specs.ReleaseDate.Year() - 1
	filter := bson.D{{"year-number", lastYearNumber}}
	var year dataTypes.Year
	ctx, cancel := context.WithTimeout(ctrl.Ctx, time.Second*30)
	defer cancel()
	err = coll.FindOne(ctx, filter).Decode(&year)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			log.Println("in mongoDatabase.GetLastYearEquivalentBenchmarkScores no last year equivalent")
			return 0, 0, errorTypes.NewNoLastYearEquivalentError("in mongoDatabase.GetLastYearEquivalentBenchmarkScores no last year equivalent")
		} else {
			log.Printf("in mongoDatabase.GetLastYearEquivalentBenchmarkScores failed to find last year equivalent: %v", err)
			return 0, 0, handleMongoError(err, false, ctrl)
		}
	}

	var priceCategories dataTypes.MinMaxInt
	switch device.Brand {
	case "Apple":
		priceCategories = dataTypes.MinMaxInt{Min: dataTypes.LowEnd, Max: dataTypes.HighEnd}
	case "Google":
		priceCategories = dataTypes.MinMaxInt{Min: device.PriceCategory - 1, Max: device.PriceCategory + 1}
	}
	lastYearsDeviceScoresAndID, err := fullTextSearch(device.Name, year.ID, priceCategories, coll, ctrl)
	if err != nil {
		log.Println("in mongoDatabase.GetLastYearEquivalentBenchmarkScores fullTextSearch err: ", err)
		return 0, 0, err
	}

	if device.Brand == "Google" {
		priceCategoryDiff := math.Abs(float64(lastYearsDeviceScoresAndID.PriceCategory - device.PriceCategory))
		lastYearsDeviceScoresAndID.Benchmark.SingleCoreScore = lastYearsDeviceScoresAndID.Benchmark.SingleCoreScore * (1 - priceCategoryDiff*0.25)
		lastYearsDeviceScoresAndID.Benchmark.MultiCoreScore = lastYearsDeviceScoresAndID.Benchmark.MultiCoreScore * (1 - priceCategoryDiff*0.25)
	}

	return lastYearsDeviceScoresAndID.Benchmark.SingleCoreScore, lastYearsDeviceScoresAndID.Benchmark.MultiCoreScore, nil
}

func (mdb *MongoDatabase) SetLastYearEquivalentBenchmarkScores(device *dataTypes.Device, ctrl *dataTypes.FlowControl) error {
	if ctrl.Ctx.Err() != nil {
		log.Printf("stopping mongoDatabase.SetLastYearEquivalentBenchmarkScores: %v", ctrl.Ctx.Err())
		return ctrl.Ctx.Err()
	}

	singleCoreScore, multiCoreScore, err := mdb.GetLastYearEquivalentBenchmarkScores(device, ctrl)
	if err != nil {
		if errorTypes.IsNoLastYearEquivalentError(err) {
			return nil
		} else {
			log.Printf("in benchmarkScraper.SetLastYearEquivalentBenchmarkScores (device: %v) failed to get last year equivelant benchmark scores: %v", device.Name, err)
			return err
		}
	}

	minMax, err := mdb.GetValidatedAndUnvalidatedMinMaxValues(ctrl)
	if err != nil {
		log.Printf("in benchmarkScraper.SetLastYearEquivalentBenchmarkScores (device: %v) failed to get minmax: %v", device.Name, err)
		return err
	}

	device.Benchmark.SingleCoreScore = singleCoreScore
	device.Benchmark.MultiCoreScore = multiCoreScore
	device.ValidatedFinalScore = aiAnalysis.GetFinalScore(minMax.Validated, device, dataTypes.ValidatedScores)
	return nil
}
