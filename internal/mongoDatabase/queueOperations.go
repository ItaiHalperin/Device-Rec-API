package mongoDatabase

import (
	"context"
	"errors"
	"fmt"
	"github.com/ItaiHalperin/Device-Rec-API/internal/dataTypes"
	"github.com/ItaiHalperin/Device-Rec-API/internal/errorMonitoring"
	"github.com/ItaiHalperin/Device-Rec-API/internal/errorTypes"
	"github.com/ItaiHalperin/Device-Rec-API/internal/helpers"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
	"time"
)

const (
	Detail = 0
	Image  = 1
)

func (mdb *MongoDatabase) EnqueueDeviceBatch(deviceNamesAndLinks map[string][]string, ctrl *dataTypes.FlowControl) error {
	if ctrl.Ctx.Err() != nil {
		log.Printf("stopping mongoDatabase.EnqueueDeviceBatch: %v", ctrl.Ctx.Err())
		return ctrl.Ctx.Err()
	}

	queueSizeCollection := mdb.client.Database("local").Collection(QueueSizeCollection)
	queueCollection := mdb.client.Database("local").Collection(QueueCollection)

	queueSize, err := mdb.GetQueueSize(queueSizeCollection, ctrl)
	if err != nil {
		log.Printf("in mongoDatabase.EnqueueDeviceBatch error getting queue size: %v", err)
		return err
	}
	remainingSpace := MaxQueueSize - queueSize

	if remainingSpace == 0 {
		return nil
	}
	err = mdb.excludeAllExistingDevices(deviceNamesAndLinks, ctrl)
	if err != nil {
		log.Printf("in mongoDatabase.EnqueueDeviceBatch error excluding existing devices: %v", err)
		return err
	}
	deviceSubset := helpers.GetSubMap(deviceNamesAndLinks, remainingSpace)

	var devicesForUploadToQueue []interface{}

	for deviceName, detailAndImage := range deviceSubset {
		devicesForUploadToQueue = append(devicesForUploadToQueue, dataTypes.DeviceInQueue{Name: deviceName, Detail: detailAndImage[Detail], Image: detailAndImage[Image]})
	}
	ctx, cancel := context.WithTimeout(ctrl.Ctx, time.Second*30)
	defer cancel()
	_, err = queueCollection.InsertMany(ctx, devicesForUploadToQueue)
	if err != nil {
		log.Printf("in mongoDatabase.EnqueueDeviceBatch error inserting devices: %v", err)
		return handleMongoError(err, false, ctrl)
	}

	err = mdb.updateQueueSize(queueSizeCollection, queueSize, len(devicesForUploadToQueue), ctrl)
	if err != nil {
		log.Printf("in mongoDatabase.EnqueueDeviceBatch error updating queue size: %v", err)
		return err
	}
	log.Printf("successfully enqueued %v", helpers.GetKeys(deviceSubset))
	return nil
}

// Dequeue removes the oldest document from the queue
func (mdb *MongoDatabase) Dequeue(ctrl *dataTypes.FlowControl) (dataTypes.DeviceInQueue, error) {
	queueCollection := mdb.client.Database(Database).Collection(QueueCollection)
	queueSizeCollection := mdb.client.Database(Database).Collection(QueueSizeCollection)
	var result dataTypes.DeviceInQueue
	ctx, cancel := context.WithTimeout(ctrl.Ctx, time.Second*10)
	defer cancel()
	err := queueCollection.FindOneAndDelete(ctx, bson.D{}).Decode(&result)
	if err != nil {
		log.Printf("in mongoDatabase.Dequeue failed to dequeue: %v", err)
		if errors.Is(err, mongo.ErrNoDocuments) {
			return dataTypes.DeviceInQueue{}, errorTypes.NewMissingDocumentError(fmt.Sprintf("in mongoDatabase.Dequeue failed to dequeue: %v", err))
		} else {
			return dataTypes.DeviceInQueue{}, handleMongoError(err, true, ctrl)
		}
	}

	queueSize, err := mdb.GetQueueSize(queueSizeCollection, ctrl)
	if err != nil {
		log.Println("in mongoDatabase.Dequeue failed to get queue size")
		return dataTypes.DeviceInQueue{}, err
	}
	err = mdb.updateQueueSize(queueSizeCollection, queueSize, -1, ctrl)
	if err != nil {
		log.Println("in mongoDatabase.Dequeue failed to update queue size")
		return dataTypes.DeviceInQueue{}, err
	}

	log.Printf("in mongoDatabase.Dequeue successfully dequeued %v", result.Name)
	return result, nil
}

func (mdb *MongoDatabase) GetQueueSize(queueSizeCollection *mongo.Collection, ctrl *dataTypes.FlowControl) (int, error) {
	var Result struct {
		QueueSize int `bson:"queue-size"`
	}

	queueSizeDocumentID, err := getObjectIDFromString(QueueSizeDocumentID, ctrl)
	if err != nil {
		log.Printf("in mongoDatabase.GetQueueSize failed to get queueSizeDocumentID ID: %v", err)
		return 0, err
	}

	err = mdb.getAndDecodeDocumentByID(&Result, queueSizeDocumentID, true, queueSizeCollection, ctrl)
	if err != nil {
		log.Printf("in mongoDatabase.GetQueueSize failed to get queueSizeDocumentID: %v", err)
		return 0, err
	}

	return Result.QueueSize, nil
}

func (mdb *MongoDatabase) updateQueueSize(queueSizeCollection *mongo.Collection, originalSize, sizeOffset int, ctrl *dataTypes.FlowControl) error {
	queueSizeDocumentID, _ := primitive.ObjectIDFromHex(QueueSizeDocumentID)
	update := bson.M{
		"$set": bson.M{
			"queue-size": originalSize + sizeOffset,
		},
	}
	ctx, cancel := context.WithTimeout(ctrl.Ctx, 10*time.Second)
	defer cancel()
	_, err := queueSizeCollection.UpdateByID(ctx, queueSizeDocumentID, update)
	if err != nil {
		log.Printf("stopping mongoDatabase.updateQueueSize failed to update queue size: %v", err)
		return handleMongoError(err, true, ctrl)
	}

	log.Printf("in mongoDatabase.updateQueueSize successfully updated queue size from %v to %v", originalSize, originalSize+sizeOffset)
	return nil
}

func (mdb *MongoDatabase) excludeAllExistingDevices(deviceNamesAndLinks map[string][]string, ctrl *dataTypes.FlowControl) error {
	if ctrl.Ctx.Err() != nil {
		log.Printf("stopping mongoDatabase.excludeAllExistingDevices: %v", ctrl.Ctx.Err())
		return ctrl.Ctx.Err()
	}

	coll := mdb.client.Database(Database).Collection(DeviceDataCollection)

	allDevices, err := mdb.getAllDevices(coll, ctrl)
	if err != nil {
		log.Printf("in mongoDatabase.excludeAllExistingDevices failed to get all devices: %v", err)
		return err
	}

	for _, device := range allDevices {
		if _, ok := deviceNamesAndLinks[device.Name]; ok {
			delete(deviceNamesAndLinks, device.Name)
		}
	}

	coll = mdb.client.Database(Database).Collection(QueueCollection)
	filter := bson.M{"name": bson.M{"$exists": true}}
	projection := bson.D{{"name", 1}, {"detail", 1}}
	ctxForFind, cancelForFind := context.WithTimeout(ctrl.Ctx, time.Second*30)
	defer cancelForFind()
	cursor, err := coll.Find(ctxForFind, filter, options.Find().SetProjection(projection))
	if err != nil {
		log.Printf("in mongoDatabase.excludeAllExistingDevices failed to find by projection")
		return handleMongoError(err, true, ctrl)
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
		log.Println("in mongoDatabase.excludeAllExistingDevices failed to decode cursor")
		return handleMongoError(err, true, ctrl)
	}

	for _, result := range results {
		if _, ok := deviceNamesAndLinks[result.Name]; ok {
			delete(deviceNamesAndLinks, result.Name)
		}
	}
	return nil
}
