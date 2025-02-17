package mongoDatabase

import (
	"Device-Rec-API/internal/aiAnalysis"
	"Device-Rec-API/internal/dataTypes"
	"Device-Rec-API/internal/reviewer"
	"context"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"log"
	"time"
)

const (
	URI                         = "mongodb://localhost:27017"
	MinMaxValuesDocumentID      = "671e570d57bc90b562fd6715"
	AllYearIDsArrayDocumentID   = "6749c756c1347240b05702ca"
	AllDeviceIDsArrayDocumentID = "6749c833c1347240b05702cb"
	MaxQueueSize                = 30
	QueueSizeDocumentID         = "673f65570a8dbe79b55eaaf9"
	Database                    = "local"
	QueueCollection             = "queue"
	QueueSizeCollection         = "queue_size_counter"
	DeviceDataCollection        = "device_data"
)

type MongoDatabase struct {
	client *mongo.Client
}

func (mdb *MongoDatabase) NormalizeUnvalidatedScores(minMaxValues dataTypes.MinMaxValues, ctrl *dataTypes.FlowControl) error {
	if ctrl.Ctx.Err() != nil {
		log.Printf("stopping mongoDatabase.NormalizeUnvalidatedScores: %v", ctrl.Ctx.Err())
		return ctrl.Ctx.Err()
	}

	deviceDataCollection := mdb.client.Database("local").Collection(DeviceDataCollection)

	allDeviceIDs, err := mdb.getAllDeviceIDs(deviceDataCollection, ctrl)

	if err != nil {
		log.Println("in mongoDatabase.NormalizeUnvalidatedScores failed to get all device IDs")
		return err
	}

	for _, curDeviceID := range allDeviceIDs {
		err = mdb.normalizeDeviceScoresByID(curDeviceID, minMaxValues, deviceDataCollection, ctrl)
		if err != nil {
			log.Printf("in mongoDatabase.NormalizeUnvalidatedScores failed to normalize review scores: %v", err)
			return err
		}
	}

	return nil
}

func (mdb *MongoDatabase) normalizeDeviceScoresByID(curDeviceID primitive.ObjectID, newMinMax dataTypes.MinMaxValues,
	coll *mongo.Collection, ctrl *dataTypes.FlowControl) error {
	if ctrl.Ctx.Err() != nil {
		log.Printf("stopping mongoDatabase.normalizeDeviceScoresByID: %v", ctrl.Ctx.Err())
		return ctrl.Ctx.Err()
	}

	var curDevice dataTypes.Device
	err := mdb.getAndDecodeDocumentByID(&curDevice, curDeviceID, true, coll, ctrl)
	if err != nil {
		return err
	}

	reviewer.SetUnvalidatedNormalizedReviewScore(newMinMax, &curDevice)
	curDevice.UnvalidatedFinalScore = aiAnalysis.GetFinalScore(newMinMax, &curDevice, dataTypes.UnvalidatedScores)
	ctx, cancel := context.WithTimeout(ctrl.Ctx, time.Second*30)
	defer cancel()
	_, err = coll.ReplaceOne(ctx, bson.M{"_id": curDeviceID}, curDevice)
	if err != nil {
		log.Printf("in mongoDatabase.normalizeDeviceScoresByID failed to normalize scores of device with id '%v'", curDeviceID.String())
		return handleMongoError(err, true, ctrl)
	}
	return nil
}
