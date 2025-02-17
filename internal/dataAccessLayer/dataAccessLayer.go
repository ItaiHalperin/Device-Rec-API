package dataAccessLayer

import (
	"Device-Rec-API/internal/databaseInterface"
)

type DataAccessLayer struct {
	Database databaseInterface.DatabaseInterface
}
