package blockinfodatabase

import (
	// TODO: Uncomment for implementing StoreBlockRecord and GetBlockRecord
	"Chain/pkg/pro"
	"google.golang.org/protobuf/proto"
	"Chain/pkg/utils"
	"github.com/syndtr/goleveldb/leveldb"
)

// BlockInfoDatabase is a wrapper for a levelDB
type BlockInfoDatabase struct {
	db *leveldb.DB
}

// New returns a BlockInfoDatabase given a Config
func New(config *Config) *BlockInfoDatabase {
	db, err := leveldb.OpenFile(config.DatabasePath, nil)
	if err != nil {
		utils.Debug.Printf("Unable to initialize BlockInfoDatabase with path {%v}", config.DatabasePath)
	}
	return &BlockInfoDatabase{db: db}
}

/*
// StoreBlockRecord stores a block record in the block info database.
func (blockInfoDB *BlockInfoDatabase) StoreBlockRecord(hash string, blockRecord *BlockRecord) {
	// TODO: Implement this function
}
*/

func (blockInfoDB *BlockInfoDatabase) StoreBlockRecord(hash string, blockRecord *BlockRecord) error {
    blockRecord_new := EncodeBlockRecord(blockRecord)
    serialized, err1 := proto.Marshal(blockRecord_new)
    if err1 != nil {
        return err1
    }
    err2 := blockInfoDB.db.Put([]byte(hash), serialized, nil)
    if err2 != nil {
        return err2
    }
    return nil
}

/*
// GetBlockRecord returns a BlockRecord from the BlockInfoDatabase given
// the relevant block's hash.
func (blockInfoDB *BlockInfoDatabase) GetBlockRecord(hash string) *BlockRecord {
	// TODO: Implement this function
	return nil
}
*/ 
// GetBlockRecord returns a BlockRecord from the BlockInfoDatabase given
// the relevant block's hash.

func (blockInfoDB *BlockInfoDatabase) GetBlockRecord(hash string) *BlockRecord {
    key := []byte(hash) //  convert this hash value to a byte slice
    value, err := blockInfoDB.db.Get(key, nil)
    if err != nil {
        // Handle the error if there was a problem retrieving the block record.
        // For example, we might return nil if the block record doesn't exist.
        return nil
    }

    // Convert the byte[] returned by the database to a protobuf.
    pbr := new(pro.BlockRecord)
    // new() is a built-in function that allocates memory for a new value of the specified type 
    // and returns a pointer to that value
    if err := proto.Unmarshal(value, pbr); err != nil {
        // Handle the error if there was a problem unmarshalling the protobuf.
        return nil
    }

    // Convert the protobuf back into a BlockRecord.
    return DecodeBlockRecord(pbr)
}
