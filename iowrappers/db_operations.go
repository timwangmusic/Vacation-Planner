package iowrappers

import (
	"Vacation-planner/POI"
	"Vacation-planner/utils"
	"github.com/globalsign/mgo"
)

type CollectionHandler interface {
	Search(radius int, coordinates []float64) []POI.Place
}

// handles operations at database level
type DbHandler struct{
	Session *mgo.Session
	handlers map[string]*CollHandler
}

// handles operations at collection level
// each collection handler is associated with a collection in a database at Init
type CollHandler struct{
	session *mgo.Session
	dbName string
	collName string
}

func (dbHandler *DbHandler) CreateSession(uri string) error{
	session, err := mgo.Dial(uri)
	utils.CheckErr(err)
	dbHandler.Session = session
	return err
}

func (collHandler *CollHandler) Init(dbHandler *DbHandler, databaseName string, collectionName string) {
	collHandler.session = dbHandler.Session
	collHandler.dbName = databaseName
	collHandler.collName = collectionName
}

func (collHandler *CollHandler) GetCollection() (coll *mgo.Collection){
	coll = collHandler.session.DB(collHandler.dbName).C(collHandler.collName)
	return
}

func (collHandler *CollHandler) Search(radius int, coordinates []float64) (places []POI.Place){
	return
}
