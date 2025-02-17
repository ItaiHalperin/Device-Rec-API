package api

import (
	"context"
	"github.com/ItaiHalperin/Device-Rec-API/internal/dataAccessLayer"
	"github.com/ItaiHalperin/Device-Rec-API/internal/dataPipelineManager"
	"github.com/ItaiHalperin/Device-Rec-API/internal/dataTypes"
	"github.com/ItaiHalperin/Device-Rec-API/internal/errorMonitoring"
	"github.com/ItaiHalperin/Device-Rec-API/internal/mongoDatabase"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
	"os"
	"time"
)

// @title SimpleWeb API
// @version 1.0
// @description API for SimpleWeb application
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.url http://www.swagger.io/support
// @contact.email support@swagger.io

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @host localhost:8080
// @BasePath /api/v1
// @schemes http
type ServerCtrl struct {
	ServerShutdownChannel <-chan os.Signal
}

// PingExample godoc
// @Summary Ping example
// @Schemes
// @Description Do Ping
// @Tags example
// @Accept  json
// @Produce  json
// @Success 200 {string} string "pong"
// @Router /api/v1/ping [get]
func Ping(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "pong"})
}

// GetUser @Summary Get a user by ID
// @Description Get details of a user by their ID
// @Tags user
// @Param id path int true "User ID"
// @Success 200 {string} string "OK"
// @Failure 404 {string} string "User not found"
// @Router /api/v1/user/{id} [get]
func GetUser(c *gin.Context) {
	id := c.Param("id")
	c.JSON(http.StatusOK, gin.H{"user": id})
}

// @Summary LaunchProcess launch the data gathering process
// @Schemes
// @Description Do launch the data gathering process
// @Tags process
// @Accept  json
// @Produce  json
// @Success 200 {string} string "pong"
// @Router /api/v1/launchProcess [get]
func (service *ServerCtrl) LaunchProcess(c *gin.Context) {
	ctrl := dataTypes.FlowControl{Ctx: context.Background(), StopOnTooManyErrorsChannel: make(chan<- struct{})}
	err := errorMonitoring.ResetErrorCounters(&ctrl)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "failed to reset monitoring"})
		return
	}

	channelForConnect := make(chan struct{}, 1)
	ctxForConnection, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()
	ctrlForConnection := dataTypes.FlowControl{
		Ctx:                        ctxForConnection,
		StopOnTooManyErrorsChannel: channelForConnect,
	}
	var dal dataAccessLayer.DataAccessLayer
	dal = dataAccessLayer.DataAccessLayer{}
	database := &mongoDatabase.MongoDatabase{}
	err = database.Connect(&ctrlForConnection)
	if err != nil {
		log.Println(err)
		c.JSON(http.StatusOK, gin.H{"message": "failed to connect to database"})
		return
	}
	dal.Database = database
	stopChannel, cancelFunc := dataPipelineManager.LaunchDataCollectionProcess(dal, dal)
	select {
	case <-stopChannel:
		log.Println("canceling context...")
		cancelFunc()
		c.JSON(http.StatusOK, gin.H{"message": "finished due to too many errors"})
	case <-service.ServerShutdownChannel:
		cancelFunc()
	}

}

// @Summary ResetDatabase reset the database
// @Schemes
// @Description Reset the device databse
// @Tags process
// @Accept  json
// @Produce  json
// @Success 200 {string} string "pong"
// @Router /api/v1/resetDatabase [get]
func ResetDatabase(c *gin.Context) {
	resetCtx, cancelReset := context.WithTimeout(context.Background(), time.Second*2)
	defer cancelReset()
	ctrl := dataTypes.FlowControl{Ctx: resetCtx, StopOnTooManyErrorsChannel: make(chan<- struct{})}
	err := errorMonitoring.ResetErrorCounters(&ctrl)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "failed to reset monitoring"})
		return
	}

	channelForConnection := make(chan struct{}, 1)
	ctxForConnection, cancelConnection := context.WithTimeout(context.Background(), time.Second*2)
	defer cancelConnection()
	ctrlForConnection := dataTypes.FlowControl{
		Ctx:                        ctxForConnection,
		StopOnTooManyErrorsChannel: channelForConnection,
	}
	database := &mongoDatabase.MongoDatabase{}
	err = database.Connect(&ctrlForConnection)
	if err != nil {
		log.Println(err)
		c.JSON(http.StatusOK, gin.H{"message": "failed to connect to database"})
		return
	}
	err = database.ResetDatabase(&ctrl)
	if err != nil {
		log.Println(err)
		c.JSON(http.StatusOK, gin.H{"message": "failed to reset database"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "successfully reset database"})
}

// @Summary Top 3 Devices
// @Description Returns the top 3 devices based on filters
// @Tags process
// @Param Filters body dataTypes.Filters true "Filters JSON"
// @Success 200 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v1/top-devices [get]
func TopDevices(c *gin.Context) {
	var filters dataTypes.Filters
	if err := c.ShouldBindJSON(&filters); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid input", "error": err.Error()})
		return
	}

	ctrl := dataTypes.FlowControl{Ctx: context.Background(), StopOnTooManyErrorsChannel: make(chan<- struct{})}

	channelForConnect := make(chan struct{}, 1)
	ctrlForConnection := dataTypes.FlowControl{
		Ctx:                        context.TODO(),
		StopOnTooManyErrorsChannel: channelForConnect,
	}
	database := &mongoDatabase.MongoDatabase{}
	err := database.Connect(&ctrlForConnection)
	if err != nil {
		log.Println(err)
		c.JSON(http.StatusOK, gin.H{"message": "failed to connect to database"})
		return
	}

	devices, err := database.GetTop3(&filters, &ctrl)

	c.JSON(http.StatusOK, gin.H{"devices": devices})
}
