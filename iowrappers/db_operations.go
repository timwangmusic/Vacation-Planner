package iowrappers

import (
	"Vacation-planner/POI"
	"Vacation-planner/utils"
	"fmt"
	"github.com/globalsign/mgo"
	"github.com/globalsign/mgo/bson"
	log "github.com/sirupsen/logrus"
)

type DatabaseHandler interface {
	PlaceSearch(req *PlaceSearchRequest) ([]POI.Place, error)
}

type CollectionHandler interface {
	Search(radius uint, latitude float64, longitude float64) []POI.Place
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
// Prevent accidentally creating collections in PlaceSearch method.
// Since the nearby search in Redis has considered maximum search radius, in this method we only need to use the updated
// search radius to search one more time in database.
func (dbHandler *DbHandler) PlaceSearch(req *PlaceSearchRequest) (places []POI.Place, err error) {
	collName := string(req.PlaceCat)

	if _, exist := dbHandler.handlers[collName]; !exist {
		err = fmt.Errorf("Collection %s does not exist", collName)
		return
	}

	collHandler := dbHandler.handlers[collName]

	totalNumDocs, err := collHandler.GetCollection().Count()
	utils.CheckErr(err)

	if uint(totalNumDocs) < req.MinNumResults {
		log.Errorf("The number of documents in database %d is less than the minimum %d requested",
			totalNumDocs, req.MinNumResults)
		return
	}

	lat_lng := utils.ParseLocation(req.Location)
	lat := lat_lng[0]
	lng := lat_lng[1]

	searchRadius := req.Radius
	places = collHandler.Search(searchRadius, lat, lng)

	return
}

func (dbHandler *DbHandler) InsertPlace(place POI.Place, placeCat POI.PlaceCategory) error {
	collName := string(placeCat)
	if _, exist := dbHandler.handlers[collName]; !exist {
		return fmt.Errorf("Collection %s does not exist", collName)
	}
	collHandler := dbHandler.handlers[collName]
	return collHandler.InsertPlace(place)
}

func (collHandler *CollHandler) Init(dbHandler *DbHandler, databaseName string, collectionName string) {
	collHandler.session = dbHandler.Session
	collHandler.dbName = databaseName
	collHandler.collName = collectionName
}

func (collHandler *CollHandler) GetCollection() (coll *mgo.Collection) {
	coll = collHandler.session.DB(collHandler.dbName).C(collHandler.collName)
	return
}

func (collHandler *CollHandler) InsertPlace(place POI.Place) error {
	err := collHandler.GetCollection().Insert(place)
	return err
}

// MongoDB geo-spatial search
// Need to create 2d sphere index before use, e.g. db.Eatery.createIndex({ location: "2dsphere" })
func (collHandler *CollHandler) Search(radius uint, latitude float64, longitude float64) (places []POI.Place) {
	query := bson.M{
		"location": bson.M{
			"$nearSphere": bson.M{
				"$geometry": bson.M{
					"type": "Point",
					// per MongoDB geoJSON requirements, specify the longitude first and then latitude
					"coordinates": [2]float64{longitude, latitude},
				},
				"$maxDistance": radius,
			},
		},
	}
	coll := collHandler.GetCollection()
	utils.CheckErr(coll.Find(query).All(&places))
	return
}
