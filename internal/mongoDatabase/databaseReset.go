package mongoDatabase

import (
	"DeviceRecommendationProject/internal/dataTypes"
	"DeviceRecommendationProject/internal/helpers"
	"context"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"log"
	"time"
)

func (mdb *MongoDatabase) ResetDatabase(ctrl *dataTypes.FlowControl) error {
	if ctrl.Ctx.Err() != nil {
		log.Printf("stopping mongoDatabase.ResetDatabase: %v", ctrl.Ctx.Err())
		return ctrl.Ctx.Err()
	}

	err := mdb.DeleteAllDocuments(ctrl)
	if err != nil {
		log.Printf("error deleting all documents: %v", err)
		return err
	}

	err = mdb.ResetMinMax(ctrl)
	if err != nil {
		log.Printf("error reseting minmax: %v", err)
		return err
	}

	err = mdb.ResetAllDeviceIDsArray(ctrl)
	if err != nil {
		log.Printf("error reseting all device ids array: %v", err)
		return err
	}

	err = mdb.ResetAllYearIDsArray(ctrl)
	if err != nil {
		log.Printf("error reseting all year ids array: %v", err)
		return err
	}

	log.Println("in mongoDatabase.ResetDatabase successfully reset database")
	return nil
}

func (mdb *MongoDatabase) ResetAllYearIDsArray(ctrl *dataTypes.FlowControl) error {
	coll := mdb.client.Database(Database).Collection(DeviceDataCollection)
	minMaxValuedDocumentID, err := getObjectIDFromString(AllYearIDsArrayDocumentID, ctrl)
	if err != nil {
		log.Println("in mongoDatabase.ResetAllYearIDsArray AllYearIDsArrayDocumentID is invalid")
		return err
	}

	update := bson.D{{"$set", bson.D{{"year-ids", make([]primitive.ObjectID, 0)}}}}
	ctx, cancel := context.WithTimeout(ctrl.Ctx, time.Second*30)
	defer cancel()
	_, err = coll.UpdateByID(ctx, minMaxValuedDocumentID, update)
	if err != nil {
		err = handleMongoError(err, true, ctrl)
		log.Printf("in mongoDatabase.ResetAllYearIDsArray failed to update year ids document: %v", err)
		return err
	}

	log.Printf("in mongoDatabase.ResetAllDeviceIDsArray reset year ids document successfully")
	return nil
}

func (mdb *MongoDatabase) DeleteAllDocuments(ctrl *dataTypes.FlowControl) error {
	coll := mdb.client.Database(Database).Collection(DeviceDataCollection)

	fields := []string{"final-score", "year", "year-number"}
	var conditions []bson.M
	for _, field := range fields {
		conditions = append(conditions, bson.M{field: bson.M{"$exists": true}})
	}
	filter := bson.M{"$or": conditions}
	ctx, cancel := context.WithTimeout(ctrl.Ctx, time.Minute)
	defer cancel()
	_, err := coll.DeleteMany(ctx, filter)
	if err != nil {
		err = handleMongoError(err, true, ctrl)
		log.Printf("in mongoDatabase.DeleteAllDevices failed to delete: %v", err)
		return err
	}

	log.Printf("in mongoDatabase.DeleteAllDevices deleted all devices successfully")
	return nil
}

func (mdb *MongoDatabase) ResetMinMax(ctrl *dataTypes.FlowControl) error {
	coll := mdb.client.Database(Database).Collection(DeviceDataCollection)

	minMaxValuedDocumentID, err := getObjectIDFromString(MinMaxValuesDocumentID, ctrl)
	if err != nil {
		log.Println("in mongoDatabase.ResetMinMax MinMaxValuesDocumentID is invalid")
		return err
	}

	defaultMinMax := helpers.GetDefaultMinMax()
	update := bson.D{{"$set", bson.D{{"validated", defaultMinMax}, {"unvalidated", defaultMinMax}}}}
	ctx, cancel := context.WithTimeout(ctrl.Ctx, time.Second*30)
	defer cancel()
	_, err = coll.UpdateByID(ctx, minMaxValuedDocumentID, update)
	if err != nil {
		err = handleMongoError(err, true, ctrl)
		log.Printf("in mongoDatabase.ResetMinMax failed to update minmax document: %v", err)
		return err
	}

	log.Printf("in mongoDatabase.ResetMinMax reseted minmax document successfully")
	return nil
}

func (mdb *MongoDatabase) ResetAllDeviceIDsArray(ctrl *dataTypes.FlowControl) error {
	coll := mdb.client.Database(Database).Collection(DeviceDataCollection)
	minMaxValuedDocumentID, err := getObjectIDFromString(AllDeviceIDsArrayDocumentID, ctrl)
	if err != nil {
		log.Println("in mongoDatabase.ResetAllDeviceIDsArray AllDeviceIDsArrayDocumentID is invalid")
		return err
	}

	update := bson.D{{"$set", bson.D{{"device-ids", make([]primitive.ObjectID, 0)}}}}
	ctx, cancel := context.WithTimeout(ctrl.Ctx, time.Second*30)
	defer cancel()
	_, err = coll.UpdateByID(ctx, minMaxValuedDocumentID, update)
	if err != nil {
		err = handleMongoError(err, true, ctrl)
		log.Printf("in mongoDatabase.ResetAllDeviceIDsArray failed to update device ids document: %v", err)
		return err
	}

	log.Printf("in mongoDatabase.ResetAllDeviceIDsArray reset device ids document successfully")
	return nil
}
