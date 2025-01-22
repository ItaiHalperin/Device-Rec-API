package mongoDatabase

import (
	"SimpleWeb/internal/dataTypes"
	"SimpleWeb/internal/errorMonitoring"
	"SimpleWeb/internal/errorTypes"
	"context"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
	"time"
)

func (mdb *MongoDatabase) GetTop3(filters *dataTypes.Filters, ctrl *dataTypes.FlowControl) ([]dataTypes.Device, error) {
	if ctrl.Ctx.Err() != nil {
		log.Printf("stopping mongoDatabase.GetTop3: %v", ctrl.Ctx.Err())
		return nil, ctrl.Ctx.Err()
	}
	coll := mdb.client.Database(Database).Collection(DeviceDataCollection)

	filter := bson.M{
		"real-price":         bson.M{"$gte": filters.Price.Min, "$lte": filters.Price.Max},
		"specs.display-size": bson.M{"$gte": filters.DisplaySize.Min, "$lte": filters.DisplaySize.Max},
		"specs.refresh-rate": bson.M{"$gte": filters.RefreshRate.Min, "$lte": filters.RefreshRate.Max},
		"brand":              bson.M{"$in": filters.Brands},
	}

	searchOptions := options.Find().SetLimit(3).SetSort(bson.M{"validated-final-score": -1})
	ctxForFind, cancelForFind := context.WithTimeout(ctrl.Ctx, time.Second*30)
	defer cancelForFind()
	cursor, err := coll.Find(ctxForFind, filter, searchOptions)
	if err != nil {
		err = handleMongoError(err, false, ctrl)
		log.Printf("in mongoDatabase.GetTop3 failed to find devices: %v", err)
		return nil, err
	}
	ctxForClose, cancelForClose := context.WithTimeout(ctrl.Ctx, time.Second*30)
	defer cancelForClose()
	defer func(cursor *mongo.Cursor, ctx context.Context) {
		err = cursor.Close(ctx)
		if err != nil {
			log.Printf("WARNING: Failed to close cursor: %v", err)
			errorMonitoring.IncrementError(errorMonitoring.CleanUpError, ctrl)
		}
	}(cursor, ctxForClose)
	ctxForDecode, cancelForDecode := context.WithTimeout(ctrl.Ctx, 10*time.Second)
	defer cancelForDecode()
	var results []dataTypes.Device
	if err = cursor.All(ctxForDecode, &results); err != nil {
		log.Println("in mongoDatabase.GetTop3 adding cursor results to devices array failed")
		return nil, handleMongoError(err, true, ctrl)
	}

	return results, err
}

type Document struct {
	ID            primitive.ObjectID `bson:"_id"`
	Score         float64            `bson:"score,omitempty"`     // Text search score
	Benchmark     Benchmark          `bson:"benchmark,omitempty"` // Nested benchmark object
	PriceCategory int                `bson:"price-category,omitempty"`
}

func fullTextSearch(modelName string, yearID primitive.ObjectID, priceCategories dataTypes.MinMaxInt, coll *mongo.Collection, ctrl *dataTypes.FlowControl) (Document, error) {
	if ctrl.Ctx.Err() != nil {
		log.Printf("stopping mongoDatabase.fullTextSearch: %v", ctrl.Ctx.Err())
		return Document{}, ctrl.Ctx.Err()
	}

	filter := bson.D{
		{Key: "$text", Value: bson.D{
			{Key: "$search", Value: modelName},
		}},
		{Key: "year", Value: yearID},
	}

	opts := options.Find().SetProjection(bson.D{
		{Key: "score", Value: bson.D{{Key: "$meta", Value: "textScore"}}},
		{Key: "_id", Value: 1},
		{Key: "benchmark", Value: 1},
		{Key: "price-category", Value: 1},
	}).SetSort(bson.D{
		{Key: "score", Value: bson.D{
			{Key: "$meta", Value: "textScore"},
		}},
	})

	ctxForFind, cancelForFind := context.WithTimeout(ctrl.Ctx, time.Second*30)
	defer cancelForFind()
	cursor, err := coll.Find(ctxForFind, filter, opts)
	if err != nil {
		err = handleMongoError(err, false, ctrl)
		log.Printf("in mongoDatabase.fullTextSearch failed to search for documents: %v", err)
		return Document{}, err
	}
	ctxForClose, cancelForClose := context.WithTimeout(ctrl.Ctx, time.Second*30)
	defer cancelForClose()
	defer func(cursor *mongo.Cursor, ctx context.Context) {
		err = cursor.Close(ctx)
		if err != nil {
			log.Printf("WARNING: Failed to close cursor: %v", err)
			errorMonitoring.IncrementError(errorMonitoring.CleanUpError, ctrl)
		}
	}(cursor, ctxForClose)

	ctxForDecode, cancelForDecode := context.WithTimeout(ctrl.Ctx, 10*time.Second)
	defer cancelForDecode()
	var devices []Document
	if err = cursor.All(ctxForDecode, &devices); err != nil {
		log.Println("in mongoDatabase.fullTextSearch adding cursor results to devices array failed")
		return Document{}, handleMongoError(err, true, ctrl)
	}

	if len(devices) == 0 {
		log.Println("in mongoDatabase.fullTextSearch no last year equivalent")
		return Document{}, errorTypes.NewNoLastYearEquivalentError("no last year equivalent")
	}

	for _, device := range devices {
		if priceCategories.Min <= device.PriceCategory && device.PriceCategory <= priceCategories.Max {
			return device, nil
		}
	}

	log.Println("in mongoDatabase.fullTextSearch no last year equivalent")
	return Document{}, errorTypes.NewNoLastYearEquivalentError("no last year equivalent")
}
