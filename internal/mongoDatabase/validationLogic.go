package mongoDatabase

import (
	"context"
	"github.com/ItaiHalperin/Device-Rec-API/dataTypes"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"log"
	"time"
)

//Should add to document where en error occurred, if one did, so that next time we know where to validate from.
//We should send updates whenever we validate a phone, so that when the server disconnects, the last phone we validated is the starting point

func (mdb *MongoDatabase) ValidateScores(unvalidatedMinMax dataTypes.MinMaxValues, ctrl *dataTypes.FlowControl) error {
	if ctrl.Ctx.Err() != nil {
		log.Printf("stopping mongoDatabase.ValidateScores: %v", ctrl.Ctx.Err())
		return ctrl.Ctx.Err()
	}
	unfinishedValidationDoc := dataTypes.ValidationFlag{IsUnfinishedValidation: true}
	ctxForInsert, cancelForInsert := context.WithTimeout(ctrl.Ctx, time.Second*30)
	defer cancelForInsert()
	log.Printf("adding unfinished validation flag to database...")
	coll := mdb.client.Database(Database).Collection(DeviceDataCollection)
	insertResult, err := coll.InsertOne(ctxForInsert, unfinishedValidationDoc)
	if err != nil {
		log.Println("in MongoDatabase.ValidateScores failed to insert unfinished validation flag")
		return handleMongoError(err, false, ctrl)
	}

	err = mdb.validateMinMaxValuesDocument(unvalidatedMinMax, coll, ctrl)
	if err != nil {
		log.Println("in mongoDatabase.ValidateScores failed to validate minmax document")
		return err
	}

	toBeValidatedDevices, err := mdb.getAllDevices(coll, ctrl)
	if err != nil {
		log.Println("in mongoDatabase.ValidateScores failed get all devices for validation")
		return err
	}

	for _, device := range toBeValidatedDevices {
		err = validateDeviceScores(device, coll, ctrl)
		if err != nil {
			log.Println("in mongoDatabase.ValidateScores failed to validate device scores")
			return err
		}
	}
	ctxForDelete, cancelForDelete := context.WithTimeout(ctrl.Ctx, time.Second*30)
	defer cancelForDelete()
	_, err = coll.DeleteOne(ctxForDelete, bson.M{"_id": insertResult.InsertedID})
	if err != nil {
		log.Println("in MongoDatabase.ValidateScores failed to delete unfinished validation flag")
		return handleMongoError(err, false, ctrl)
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
		log.Println("in mongoDatabase.ValidateScores MinMaxValuesDocumentID is invalid")
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
