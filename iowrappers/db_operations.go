package iowrappers

import (
	"Vacation-planner/POI"
	"Vacation-planner/utils"
	"fmt"
	"github.com/globalsign/mgo"
	"github.com/globalsign/mgo/bson"
	"strconv"
	"strings"
)

type DatabaseHandler interface {
	PlaceSearch(req *PlaceSearchRequest) ([]POI.Place, error)
}

type CollectionHandler interface {
	Search(radius uint, coordinates []float64) []POI.Place
}

// handles operations at database level
// DbHandler.handlers manages collection handlers
type DbHandler struct {
	dbName   string
	Session  *mgo.Session
	handlers map[string]*CollHandler
}

// handles operations at collection level
// each collection handler is associated with a collection in a database at Init
type CollHandler struct {
	session  *mgo.Session
	dbName   string
	collName string
}

func (dbHandler *DbHandler) Init(DbName string, url string) {
	dbHandler.handlers = make(map[string]*CollHandler)
	dbHandler.dbName = DbName
	dbHandler.CreateSession(url)
}

func (dbHandler *DbHandler) CreateSession(uri string) {
	session, err := mgo.Dial(uri)
	utils.CheckErr(err)
	dbHandler.Session = session
}

func (dbHandler *DbHandler) SetCollHandler(collectionName string) {
	if _, exist := dbHandler.handlers[collectionName]; !exist {
		collHandler := &CollHandler{}
		dbHandler.handlers[collectionName] = collHandler
		collHandler.Init(dbHandler, dbHandler.dbName, collectionName)
	}
}

// This design make sure that explicit call to SetCollHandler have to be made for new collection creation.
// Prevent accidentally creating collections in PlaceSearch method
func (dbHandler *DbHandler) PlaceSearch(req *PlaceSearchRequest) ([]POI.Place, error) {
	collName := string(req.PlaceCat)
	if _, exist := dbHandler.handlers[collName]; !exist {
		return nil, fmt.Errorf("Collection %s does not exist", collName)
	}
	collHandler := dbHandler.handlers[collName]
	radius := req.Radius
	coordinates := strings.Split(req.Location, ",")
	lat, _ := strconv.ParseFloat(coordinates[0], 64)
	lng, _ := strconv.ParseFloat(coordinates[1], 64)
	return collHandler.Search(radius, []float64{lat, lng}), nil
}

func (dbHandler *DbHandler) InsertPlace(place POI.Place, placeCat POI.PlaceCategory) error {
	collName := string(placeCat)
	if _, exist := dbHandler.handlers[collName]; !exist {
		return fmt.Errorf("Collection %s does not exist", collName)
	}
	collHandler := dbHandler.handlers[collName]
	collHandler.InsertPlace(place)
	return nil
}

func (collHandler *CollHandler) Init(dbHandler *DbHandler, databaseName string, collectionName string) {
	collHandler.session = dbHandler.Session
	collHandler.dbName = databaseName
	collHandler.collName = collectionName
	// TODO: Ensure placeId index at initialization
}

func (collHandler *CollHandler) GetCollection() (coll *mgo.Collection) {
	coll = collHandler.session.DB(collHandler.dbName).C(collHandler.collName)
	return
}

func (collHandler *CollHandler) InsertPlace(place POI.Place) {
	err := collHandler.GetCollection().Insert(place)
	utils.CheckErr(err)
}

// MongoDB geo-spatial search
// Need to create 2d sphere index before use, e.g. db.Eatery.createIndex({ location: "2dsphere" })
func (collHandler *CollHandler) Search(radius uint, coordinates []float64) (places []POI.Place) {
	lat := coordinates[0]
	lng := coordinates[1]
	query := bson.M{
		"location": bson.M{
			"$nearSphere": bson.M{
				"$geometry": bson.M{
					"type":        "Point",
					// per MongoDB geoJSON requirements, specify the longitude first and then latitude
					"coordinates": [2]float64{lng, lat},
				},
				"$maxDistance": radius,
			},
		},
	}
	coll := collHandler.GetCollection()
	utils.CheckErr(coll.Find(query).All(&places))
	return
}
