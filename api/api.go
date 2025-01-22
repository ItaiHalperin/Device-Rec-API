package api

import (
	"SimpleWeb/internal/dataAccessLayer"
	"SimpleWeb/internal/dataPipelineManager"
	"SimpleWeb/internal/dataTypes"
	"SimpleWeb/internal/errorMonitoring"
	"SimpleWeb/internal/mongoDatabase"
	"context"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
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

type PixelModel struct {
	Model       string `json:"model"`
	Price       int    `json:"price"`
	ReleaseDate string `json:"release_date"`
	Name        string `json:"name"`
	Rating      int    `json:"rating"`
}

type PixelModelsData struct {
	GooglePixelModels []PixelModel `json:"google_pixel_models"`
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
func LaunchProcess(c *gin.Context) {
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
		log.Fatal(err)
	}
	dal.Database = database
	stopChannel, cancelFunc := dataPipelineManager.LaunchDataCollectionProcess(dal, dal)
	_ = <-stopChannel
	log.Println("canceling context...")
	cancelFunc()
	c.JSON(http.StatusOK, gin.H{"message": "finished due to too many errors"})
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
	ctrl := dataTypes.FlowControl{Ctx: context.Background(), StopOnTooManyErrorsChannel: make(chan<- struct{})}
	err := errorMonitoring.ResetErrorCounters(&ctrl)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"message": "failed to reset monitoring"})
		return
	}

	channelForConnect := make(chan struct{}, 1)
	ctrlForConnection := dataTypes.FlowControl{
		Ctx:                        context.TODO(),
		StopOnTooManyErrorsChannel: channelForConnect,
	}
	database := &mongoDatabase.MongoDatabase{}
	err = database.Connect(&ctrlForConnection)
	if err != nil {
		log.Fatal(err)
	}
	err = database.ResetDatabase(&ctrl)
	if err != nil {
		log.Fatal(err)
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
		log.Fatal(err)
	}

	devices, err := database.GetTop3(&filters, &ctrl)

	c.JSON(http.StatusOK, gin.H{"devices": devices})
}
