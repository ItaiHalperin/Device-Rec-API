package mongoDatabase

import (
	"DeviceRecommendationProject/internal/dataTypes"
	"DeviceRecommendationProject/internal/errorMonitoring"
	"context"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"log"
	"time"
)

//Should add to document where en error occurred, if one did, so that next time we know where to validate from.
//We should send updates whenever we validate a phone, so that when the server disconnects, the last phone we validated is the starting point

func (mdb *MongoDatabase) validateScores(unvalidatedMinMax dataTypes.MinMaxValues, coll *mongo.Collection, ctrl *dataTypes.FlowControl) error {
	if ctrl.Ctx.Err() != nil {
		log.Printf("stopping mongoDatabase.validateScores: %v", ctrl.Ctx.Err())
		return ctrl.Ctx.Err()
	}

	err := mdb.validateMinMaxValuesDocument(unvalidatedMinMax, coll, ctrl)
	if err != nil {
		log.Println("in mongoDatabase.validateScores failed to validate minmax document")
		return err
	}

	toBeValidatedDevices, err := mdb.getAllDevices(coll, ctrl)
	if err != nil {
		log.Println("in mongoDatabase.validateScores failed get all devices for validation")
		return err
	}

	if len(toBeValidatedDevices) != unvalidatedMinMax.NumberOfDevices {
		log.Println("in mongoDatabase.validateScores ailed to find all devices in database")
		errorMonitoring.IncrementError(errorMonitoring.MissingDocumentError, ctrl)
	}

	for _, device := range toBeValidatedDevices {
		err = validateDeviceScores(device, coll, ctrl)
		if err != nil {
			log.Println("in mongoDatabase.validateScores failed to validate device scores")
			return err
		}
	}
	return nil
}

func (mdb *MongoDatabase) validateMinMaxValuesDocument(unvalidatedMinMax dataTypes.MinMaxValues, coll *mongo.Collection, ctrl *dataTypes.FlowControl) error {
	if ctrl.Ctx.Err() != nil {
		log.Printf("stopping mongoDatabase.validateMinMaxValuesDocument: %v", ctrl.Ctx.Err())
		return ctrl.Ctx.Err()
	}

	minMaxValuedDocumentID, err := getObjectIDFromString(MinMaxValuesDocumentID, ctrl)
	if err != nil {
		log.Println("in mongoDatabase.validateScores MinMaxValuesDocumentID is invalid")
		return err
	}

	update := bson.D{{"$set", bson.D{{"validated", unvalidatedMinMax}}}}
	ctx, cancel := context.WithTimeout(ctrl.Ctx, time.Second*30)
	defer cancel()
	_, err = coll.UpdateByID(ctx, minMaxValuedDocumentID, update)
	if err != nil {
		log.Println("in mongoDatabase.validateMinMaxValuesDocument failed to update minmax document")
		return handleMongoError(err, true, ctrl)
	}

	return nil
}

func validateDeviceScores(device dataTypes.Device, coll *mongo.Collection, ctrl *dataTypes.FlowControl) error {
	if ctrl.Ctx.Err() != nil {
		log.Printf("stopping mongoDatabase.validateDeviceScores: %v", ctrl.Ctx.Err())
		return ctrl.Ctx.Err()
	}

	update := bson.D{{"$set", bson.D{{"review.validated-review-score", device.Review.UnvalidatedReviewScore},
		{"validated-final-score", device.UnvalidatedFinalScore}}}}

	ctx, cancel := context.WithTimeout(ctrl.Ctx, time.Second*30)
	defer cancel()
	_, err := coll.UpdateByID(ctx, device.ID, update)
	if err != nil {
		log.Println("in mongoDatabase.validateDeviceScores failed tp update device")
		return handleMongoError(err, true, ctrl)
	}
	return nil
}
