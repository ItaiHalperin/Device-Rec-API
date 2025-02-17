package dataAccessLayer

import (
	"github.com/ItaiHalperin/Device-Rec-API/internal/databaseInterface"
)

type DataAccessLayer struct {
	Database databaseInterface.DatabaseInterface
}
