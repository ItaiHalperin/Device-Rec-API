package dataAccessLayer

import (
	"DeviceRecommendationProject/internal/databaseInterface"
)

type DataAccessLayer struct {
	Database databaseInterface.DatabaseInterface
}
