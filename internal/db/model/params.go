package model

type GlobalParamsDocument struct {
	Type    string      `bson:"type"`
	Version uint32      `bson:"version"`
	Params  interface{} `bson:"params"`
}
