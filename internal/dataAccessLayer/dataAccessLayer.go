package dataAccessLayer

import (
	"SimpleWeb/internal/databaseInterface"
)

type DataAccessLayer struct {
	Database databaseInterface.DatabaseInterface
}
