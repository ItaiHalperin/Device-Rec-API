package mongoDatabase

import (
	"DeviceRecommendationProject/internal/dataTypes"
	"DeviceRecommendationProject/internal/errorMonitoring"
	"DeviceRecommendationProject/internal/errorTypes"
	"context"
	"errors"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"log"
	"time"
)

func handleMongoError(err error, isDocumentExpectedToExist bool, ctrl *dataTypes.FlowControl) error {
	if ctrl.Ctx.Err() != nil {
		log.Printf("stopping mongoDatabase.handleMongoError: %v", ctrl.Ctx.Err())
		return ctrl.Ctx.Err()
	}

	if isDocumentExpectedToExist && errors.Is(err, mongo.ErrNoDocuments) {
		errorMonitoring.IncrementError(errorMonitoring.MissingDocumentError, ctrl)
		return errorTypes.NewMissingDocumentError("failed to find document")
	}
	if mongo.IsNetworkError(err) {
		errorMonitoring.IncrementError(errorMonitoring.DatabaseNetworkError, ctrl)
		return errorTypes.NewMissingDocumentError("failed to connect to database")
	}

	errorMonitoring.IncrementError(errorMonitoring.GeneralDatabaseError, ctrl)
	return errorTypes.NewGeneralDatabaseError("failed due to database error")
}

func (mdb *MongoDatabase) getAndDecodeDocumentByID(emptyDataStructPointer interface{}, documentID primitive.ObjectID, isDocumentExpectedToExist bool, coll *mongo.Collection, ctrl *dataTypes.FlowControl) error {
	if ctrl.Ctx.Err() != nil {
		log.Printf("stopping mongoDatabase.getAndDecodeDocumentByID: %v", ctrl.Ctx.Err())
		return ctrl.Ctx.Err()
	}

	filter := bson.D{{"_id", documentID}}

	ctx, cancel := context.WithTimeout(ctrl.Ctx, 10*time.Second)
	defer cancel()

	err := coll.FindOne(ctx, filter).Decode(emptyDataStructPointer)
	if err != nil {
		log.Printf("in mongoDatabase.getAndDecodeDocumentByID failed to find document: %v", err)
		return handleMongoError(err, isDocumentExpectedToExist, ctrl)
	}

	return nil
}

func getObjectIDFromString(IDString string, ctrl *dataTypes.FlowControl) (primitive.ObjectID, error) {
	if ctrl.Ctx.Err() != nil {
		log.Printf("stopping mongoDatabase.getObjectIDFromString: %v", ctrl.Ctx.Err())
		return primitive.ObjectID{}, ctrl.Ctx.Err()
	}

	objectID, err := primitive.ObjectIDFromHex(IDString)
	if err != nil {
		errorMonitoring.IncrementError(errorMonitoring.InvalidConstIDStringError, ctrl)
		return primitive.ObjectID{}, errorTypes.NewInvalidConstIDStringError(fmt.Sprintf("const id string '%v' is invalid", IDString))
	}
	return objectID, nil
}

func (mdb *MongoDatabase) IsUp(ctrl *dataTypes.FlowControl) bool {
	if ctrl.Ctx.Err() != nil {
		log.Printf("stopping mongoDatabase.IsUp: %v", ctrl.Ctx.Err())
		return false
	}

	if mdb.client == nil {
		log.Println("in mongoDatabase.isUp database client is nil ")
		return false
	}

	ctx, cancel := context.WithTimeout(ctrl.Ctx, 10*time.Second)
	defer cancel()

	err := mdb.client.Ping(ctx, readpref.Primary())
	if err != nil {
		log.Printf("in mongoDatabase.isUp failed to ping database: %v", err)
		return false
	}

	return true
}

func (mdb *MongoDatabase) GetValidatedAndUnvalidatedMinMaxValues(ctrl *dataTypes.FlowControl) (dataTypes.ValidatedAndUnvalidatedMinMaxValues, error) {
	if ctrl.Ctx.Err() != nil {
		return dataTypes.ValidatedAndUnvalidatedMinMaxValues{}, ctrl.Ctx.Err()
	}

	coll := mdb.client.Database("local").Collection(DeviceDataCollection)

	id, _ := primitive.ObjectIDFromHex(MinMaxValuesDocumentID)
	var minMaxValues dataTypes.ValidatedAndUnvalidatedMinMaxValues
	err := mdb.getAndDecodeDocumentByID(&minMaxValues, id, true, coll, ctrl)
	if err != nil {
		log.Println("in mongoDatabase.GetValidatedAndUnvalidatedMinMaxValues failed get minmax document")
		return dataTypes.ValidatedAndUnvalidatedMinMaxValues{}, handleMongoError(err, true, ctrl)
	}

	return minMaxValues, nil
}

func (mdb *MongoDatabase) Connect(ctrl *dataTypes.FlowControl) error {
	if ctrl.Ctx.Err() != nil {
		return ctrl.Ctx.Err()
	}

	client, err := getClient(ctrl)
	if err != nil {
		log.Println("in mongoDatabase.Connect failed to connect to database")
		return err
	}

	mdb.client = client
	return nil
}

func (mdb *MongoDatabase) Disconnect(ctrl *dataTypes.FlowControl) error {
	ctx, cancel := context.WithTimeout(ctrl.Ctx, 10*time.Second)
	defer cancel()
	if err := mdb.client.Disconnect(ctx); err != nil {
		log.Printf("failed to disconnect MongoDB client: %v", err)
		return handleMongoError(err, true, ctrl)
	}
	return nil
}

func getClient(ctrl *dataTypes.FlowControl) (*mongo.Client, error) {
	client, err := mongo.Connect(ctrl.Ctx, options.Client().ApplyURI(URI))
	if err != nil {
		log.Printf("failed to connect to MongoDB: %v", err)
		return nil, handleMongoError(err, true, ctrl)
	}

	log.Println("created new client successfully")
	return client, nil
}

func setupTextIndex(collection *mongo.Collection, ctrl *dataTypes.FlowControl) error {
	model := mongo.IndexModel{
		Keys: bson.D{{Key: "name", Value: "text"}},
	}
	ctx, cancel := context.WithTimeout(ctrl.Ctx, time.Minute)
	defer cancel()
	_, err := collection.Indexes().CreateOne(ctx, model)
	if err != nil {
		log.Println("setupTextIndex failure")
		return handleMongoError(err, false, ctrl)
	}

	log.Println("setupTextIndex success")
	return nil
}

type Benchmark struct {
	MultiCoreScore  float64 `bson:"multi-core-score"`
	SingleCoreScore float64 `bson:"single-core-score"`
}

func (mdb *MongoDatabase) getAllDevices(coll *mongo.Collection, ctrl *dataTypes.FlowControl) ([]dataTypes.Device, error) {
	if ctrl.Ctx.Err() != nil {
		log.Printf("stopping mongoDatabase.getAllDevices: %v", ctrl.Ctx.Err())
		return nil, ctrl.Ctx.Err()
	}

	allDeviceIDs, err := mdb.getAllDeviceIDs(coll, ctrl)
	if err != nil {
		log.Println("in mongoDatabase.getAllDevices failed to get all device IDs")
		return nil, err
	}

	var toBeValidatedDevices []dataTypes.Device
	for _, curDeviceID := range allDeviceIDs {
		var curDevice dataTypes.Device
		err = mdb.getAndDecodeDocumentByID(&curDevice, curDeviceID, true, coll, ctrl)
		if err != nil {
			log.Println("in mongoDatabase.getAllDevices failed to get and decode device")
			return nil, err
		}
		toBeValidatedDevices = append(toBeValidatedDevices, curDevice)
	}
	return toBeValidatedDevices, nil
}

func (mdb *MongoDatabase) getAllYearIDs(coll *mongo.Collection, ctrl *dataTypes.FlowControl) ([]primitive.ObjectID, error) {
	yearsDocumentID, err := getObjectIDFromString(AllYearIDsArrayDocumentID, ctrl)
	if err != nil {
		log.Println("in mongoDatabase.getAllYearIDs yearsArrayDocumentID is invalid")
		return nil, err
	}

	var yearsDocument dataTypes.YearIDsDocument
	err = mdb.getAndDecodeDocumentByID(&yearsDocument, yearsDocumentID, true, coll, ctrl)
	if err != nil {
		log.Printf("in mongoDatabase.getAllYearIDs failed to get years document")
		return nil, err
	}
	return yearsDocument.YearIDs, nil
}

func (mdb *MongoDatabase) getAllDeviceIDs(deviceDataCollection *mongo.Collection, ctrl *dataTypes.FlowControl) ([]primitive.ObjectID, error) {
	yearsDocumentID, err := getObjectIDFromString(AllDeviceIDsArrayDocumentID, ctrl)
	if err != nil {
		log.Println("in mongoDatabase.getAllDeviceIDs AllDeviceIDsArrayDocumentID is invalid")
		return nil, err
	}

	var deviceIDsDocument dataTypes.DeviceIDsDocument
	err = mdb.getAndDecodeDocumentByID(&deviceIDsDocument, yearsDocumentID, true, deviceDataCollection, ctrl)
	if err != nil {
		log.Printf("in mongoDatabase.getAllDeviceIDs failed to get device IDs document")
		return nil, err
	}
	return deviceIDsDocument.DeviceIDs, nil
}
