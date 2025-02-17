package mongoDatabase

import (
	"context"
	"github.com/ItaiHalperin/Device-Rec-API/internal/aiAnalysis"
	"github.com/ItaiHalperin/Device-Rec-API/internal/dataTypes"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"log"
	"time"
)

func (mdb *MongoDatabase) UploadDevice(device *dataTypes.Device, unvalidatedMinMax dataTypes.MinMaxValues,
	ctrl *dataTypes.FlowControl) error {
	if ctrl.Ctx.Err() != nil {
		log.Printf("stopping mongoDatabase.UploadDevice: %v", ctrl.Ctx.Err())
		return ctrl.Ctx.Err()
	}

	coll := mdb.client.Database(Database).Collection(DeviceDataCollection)
	device.UnvalidatedFinalScore = aiAnalysis.GetFinalScore(unvalidatedMinMax, device, dataTypes.UnvalidatedScores)
	err := mdb.SetDeviceIDs(device, coll, ctrl)
	if err != nil {
		log.Printf("in mongoDatabase.UploadDevice failed to set device ID's")
		return err
	}
	ctx, cancel := context.WithTimeout(ctrl.Ctx, time.Second*30)
	defer cancel()
	log.Printf("in mongoDatabase.UploadDevice inserting %v into database", device.Name)
	_, err = coll.InsertOne(ctx, device)
	if err != nil {
		err = handleMongoError(err, false, ctrl)
		log.Printf("in mongoDatabase.validateScores failed to insert device into database: %v", err)
		return err
	}
	err = mdb.incrementUnvalidatedNumberOfDevices(&unvalidatedMinMax, coll, ctrl)
	if err != nil {
		log.Println("in mongoDatabase.validateScores failed to increment unvalidated number of devices")
		return err
	}

	err = mdb.validateScores(unvalidatedMinMax, coll, ctrl)
	if err != nil {
		log.Printf("in mongoDatabase.UploadDevice failed to upload device fully due to failed validation: %v", err)
		return err
	}
	log.Printf("in mongoDatabase.UploadDevice successfully uploaded %v into database", device.Name)
	return nil
}

func (mdb *MongoDatabase) SetDeviceIDs(device *dataTypes.Device, deviceDataCollection *mongo.Collection,
	ctrl *dataTypes.FlowControl) error {
	if ctrl.Ctx.Err() != nil {
		log.Printf("stopping mongoDatabase.SetDeviceIDs: %v", ctrl.Ctx.Err())
		return ctrl.Ctx.Err()
	}

	newDeviceID := primitive.NewObjectID()
	yearNumber := device.Specs.ReleaseDate.Year()
	monthNumber := int(device.Specs.ReleaseDate.Month())

	yearID, err := mdb.getYearID(yearNumber, deviceDataCollection, ctrl)
	if err != nil {
		log.Println("in mongoDatabase.SetDeviceIDs failed to get year")
		return err
	}
	monthID, err := mdb.getMonthID(monthNumber, yearID, deviceDataCollection, ctrl)
	if err != nil {
		log.Println("in mongoDatabase.SetDeviceIDs failed to get month")
		return err
	}

	err = mdb.addIDToDeviceArray(newDeviceID, deviceDataCollection, ctrl)
	if err != nil {
		log.Println("in mongoDatabase.SetDeviceIDs failed to add ID to devices array")
		return err
	}

	err = mdb.addIDToMonth(newDeviceID, monthID, deviceDataCollection, ctrl)
	if err != nil {
		log.Println("in mongoDatabase.SetDeviceIDs failed to add ID to month devices array")
		return err
	}

	device.Year = yearID
	device.Month = monthID
	device.ID = newDeviceID
	return nil
}

func (mdb *MongoDatabase) addIDToMonth(deviceID, monthID primitive.ObjectID, deviceDataCollection *mongo.Collection, ctrl *dataTypes.FlowControl) error {
	var month dataTypes.Month
	err := mdb.getAndDecodeDocumentByID(&month, monthID, true, deviceDataCollection, ctrl)
	if err != nil {
		log.Printf("in mongoDatabase.addIDToMonth failed get month with id %v document", monthID)
		return handleMongoError(err, true, ctrl)
	}

	month.Devices = append(month.Devices, deviceID)

	update := bson.M{
		"$set": bson.M{
			"devices": month.Devices,
		},
	}
	ctx, cancel := context.WithTimeout(ctrl.Ctx, time.Second*30)
	defer cancel()
	_, err = deviceDataCollection.UpdateByID(ctx, monthID, update)
	if err != nil {
		log.Printf("in mongoDatabase.addIDToMonth failed to update month %v's devices", month.MonthNumber)
		return handleMongoError(err, true, ctrl)
	}

	return nil
}

func (mdb *MongoDatabase) addIDToDeviceArray(deviceID primitive.ObjectID, deviceDataCollection *mongo.Collection, ctrl *dataTypes.FlowControl) error {
	if ctrl.Ctx.Err() != nil {
		log.Printf("stopping mongoDatabase.addIDToDeviceArray: %v", ctrl.Ctx.Err())
		return ctrl.Ctx.Err()
	}

	allDeviceIDs, err := mdb.getAllDeviceIDs(deviceDataCollection, ctrl)
	if err != nil {
		log.Println("in mongoDatabase.addIDToDeviceArray failed to get all year IDs")
		return err
	}
	allDeviceIDs = append(allDeviceIDs, deviceID)
	allDeviceIdsArrayDocumentID, err := getObjectIDFromString(AllDeviceIDsArrayDocumentID, ctrl)
	if err != nil {
		log.Println("in mongoDatabase.addIDToDeviceArray yearsArrayDocumentID is invalid")
		return err
	}
	update := bson.M{
		"$set": bson.M{
			"device-ids": allDeviceIDs,
		},
	}
	ctx, cancel := context.WithTimeout(ctrl.Ctx, time.Second*30)
	defer cancel()
	_, err = deviceDataCollection.UpdateByID(ctx, allDeviceIdsArrayDocumentID, update)
	if err != nil {
		log.Println("in mongoDatabase.addIDToDeviceArray failed to update device IDs document")
		return handleMongoError(err, true, ctrl)
	}

	log.Printf("in mongoDatabase.addIDToDeviceArray added ID to device IDs array: %v", deviceID)
	return nil
}

func (mdb *MongoDatabase) addIDToYearArray(yearID primitive.ObjectID, deviceDataCollection *mongo.Collection, ctrl *dataTypes.FlowControl) error {
	if ctrl.Ctx.Err() != nil {
		log.Printf("stopping mongoDatabase.addIDToYearArray: %v", ctrl.Ctx.Err())
		return ctrl.Ctx.Err()
	}

	allYearIDs, err := mdb.getAllYearIDs(deviceDataCollection, ctrl)
	if err != nil {
		log.Println("in mongoDatabase.addIDToYearArray failed to get all year IDs")
		return err
	}
	allYearIDs = append(allYearIDs, yearID)
	yearsDocumentID, err := getObjectIDFromString(AllYearIDsArrayDocumentID, ctrl)
	if err != nil {
		log.Println("in mongoDatabase.addIDToYearArray yearsArrayDocumentID is invalid")
		return err
	}
	update := bson.M{
		"$set": bson.M{
			"year-ids": allYearIDs,
		},
	}
	ctx, cancel := context.WithTimeout(ctrl.Ctx, time.Second*30)
	defer cancel()
	_, err = deviceDataCollection.UpdateByID(ctx, yearsDocumentID, update)
	if err != nil {
		log.Println("in mongoDatabase.addNewYear failed to update years document")
		return handleMongoError(err, true, ctrl)
	}
	log.Printf("in mongoDatabase.addIDToYearArray added year IDs array: %v", yearsDocumentID)
	return nil
}

func (mdb *MongoDatabase) getMonthID(monthNumber int, yearID primitive.ObjectID, coll *mongo.Collection, ctrl *dataTypes.FlowControl) (primitive.ObjectID, error) {
	if ctrl.Ctx.Err() != nil {
		log.Printf("stopping mongoDatabase.getMonthID: %v", ctrl.Ctx.Err())
		return primitive.NilObjectID, ctrl.Ctx.Err()
	}

	var year dataTypes.Year
	err := mdb.getAndDecodeDocumentByID(&year, yearID, true, coll, ctrl)
	if err != nil {
		log.Println("in MongoDatabase.getMonthID failed to gather month's year document")
		return primitive.NilObjectID, err
	}

	for _, monthID := range year.Months {
		var month dataTypes.Month
		err = mdb.getAndDecodeDocumentByID(&month, monthID, true, coll, ctrl)
		if err != nil {
			log.Println("in mongoDatabase.getMonthID failed to get one of year's month documents")
			return primitive.NilObjectID, err
		}

		if month.MonthNumber == monthNumber {
			return month.ID, nil
		}
	}

	newMonthID, err := mdb.addNewMonth(monthNumber, year, yearID, coll, ctrl)
	if err != nil {
		log.Println("in mongoDatabase.getMonthID failed to add new month")
		return primitive.NilObjectID, err
	}

	return newMonthID, nil
}

func (mdb *MongoDatabase) getYearID(yearNumber int, coll *mongo.Collection, ctrl *dataTypes.FlowControl) (primitive.ObjectID, error) {
	if ctrl.Ctx.Err() != nil {
		log.Printf("stopping mongoDatabase.getYearID: %v", ctrl.Ctx.Err())
		return primitive.ObjectID{}, ctrl.Ctx.Err()
	}

	allYearIDs, err := mdb.getAllYearIDs(coll, ctrl)
	if err != nil {
		log.Println("in mongoDatabase.getYearID failed to gather all year ids")
		return primitive.NilObjectID, err
	}

	for _, yearID := range allYearIDs {
		var year dataTypes.Year
		err = mdb.getAndDecodeDocumentByID(&year, yearID, true, coll, ctrl)
		if err != nil {
			log.Println("in mongoDatabase.getYearID failed to get year document")
			return primitive.NilObjectID, err
		}

		if year.YearNumber == yearNumber {
			return year.ID, nil
		}
	}

	newYearID, err := mdb.addNewYear(yearNumber, coll, ctrl)
	if err != nil {
		log.Println("in mongoDatabase.getYearID failed to add new year")
		return primitive.ObjectID{}, err
	}

	return newYearID, nil
}

func (mdb *MongoDatabase) addNewYear(yearNumber int, deviceDataCollection *mongo.Collection, ctrl *dataTypes.FlowControl) (primitive.ObjectID, error) {
	if ctrl.Ctx.Err() != nil {
		log.Printf("stopping mongoDatabase.addNewYear: %v", ctrl.Ctx.Err())
		return primitive.NilObjectID, ctrl.Ctx.Err()
	}

	yearID := primitive.NewObjectID()
	newYear := dataTypes.Year{
		ID:         yearID,
		YearNumber: yearNumber,
		Months:     nil,
	}
	ctx, cancel := context.WithTimeout(ctrl.Ctx, time.Second*30)
	defer cancel()
	log.Printf("adding new year %v to database...", yearNumber)
	_, err := deviceDataCollection.InsertOne(ctx, newYear)
	if err != nil {
		log.Println("in MongoDatabase.addNewYear failed to insert new year")
		return primitive.ObjectID{}, handleMongoError(err, false, ctrl)
	}

	err = mdb.addIDToYearArray(yearID, deviceDataCollection, ctrl)
	if err != nil {
		log.Println("in MongoDatabase.addNewYear failed to add id to years array")
		return primitive.ObjectID{}, err
	}

	log.Printf("in mongoDatabase.addNewYear successfully added year %v with id: %v", yearNumber, yearID)
	return yearID, nil
}

func (mdb *MongoDatabase) addNewMonth(monthNumber int, year dataTypes.Year, yearID primitive.ObjectID, coll *mongo.Collection, ctrl *dataTypes.FlowControl) (primitive.ObjectID, error) {
	if ctrl.Ctx.Err() != nil {
		log.Printf("stopping mongoDatabase.addNewMonth: %v", ctrl.Ctx.Err())
		return primitive.NilObjectID, ctrl.Ctx.Err()
	}

	monthID := primitive.NewObjectID()
	newMonth := dataTypes.Month{
		ID:          monthID,
		MonthNumber: monthNumber,
		Year:        yearID,
		Devices:     []primitive.ObjectID{},
	}
	ctxForInsert, cancelForInsert := context.WithTimeout(ctrl.Ctx, time.Second*30)
	defer cancelForInsert()
	log.Printf("adding new month %v with year %v to database...", monthNumber, year.YearNumber)
	_, err := coll.InsertOne(ctxForInsert, newMonth)
	if err != nil {
		log.Println("in MongoDatabase.addNewYear failed to insert new month")
		return primitive.ObjectID{}, handleMongoError(err, false, ctrl)
	}

	year.Months = append(year.Months, monthID)
	update := bson.D{{"$set", bson.D{{"months", year.Months}}}}
	ctxForUpdate, cancelForUpdate := context.WithTimeout(ctrl.Ctx, time.Second*30)
	defer cancelForUpdate()
	_, err = coll.UpdateByID(ctxForUpdate, yearID, update)
	if err != nil {
		log.Printf("in MongoDatabase.addNewMonth failed to update year %v with month %v", year.Months, monthNumber)
		return primitive.ObjectID{}, handleMongoError(err, true, ctrl)
	}

	return monthID, nil
}

func (mdb *MongoDatabase) incrementUnvalidatedNumberOfDevices(unvalidatedMinMax *dataTypes.MinMaxValues, deviceDataCollection *mongo.Collection, ctrl *dataTypes.FlowControl) error {
	if ctrl.Ctx.Err() != nil {
		log.Printf("stopping mongoDatabase.incrementUnvalidatedNumberOfDevices: %v", ctrl.Ctx.Err())
		return ctrl.Ctx.Err()
	}

	minMaxValuedDocumentID, err := getObjectIDFromString(MinMaxValuesDocumentID, ctrl)
	if err != nil {
		log.Println("in mongoDatabase.incrementUnvalidatedNumberOfDevices MinMaxValuesDocumentID is invalid")
		return err
	}
	unvalidatedMinMax.NumberOfDevices++
	update := bson.D{{"$set", bson.D{{"unvalidated", unvalidatedMinMax}}}}
	ctx, cancel := context.WithTimeout(ctrl.Ctx, time.Second*30)
	defer cancel()
	_, err = deviceDataCollection.UpdateByID(ctx, minMaxValuedDocumentID, update)
	if err != nil {
		log.Println("in mongoDatabase.incrementUnvalidatedNumberOfDevices failed to update minmax document")
		return handleMongoError(err, true, ctrl)
	}
	return nil
}
