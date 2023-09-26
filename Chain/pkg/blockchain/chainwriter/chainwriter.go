package chainwriter

import (
	"Chain/pkg/block"
	"Chain/pkg/blockchain/blockinfodatabase"
	"Chain/pkg/pro"
	"Chain/pkg/utils"
	"log"
	"os"
	"fmt"
	"google.golang.org/protobuf/proto"
)

// ChainWriter handles all I/O for the BlockChain. It stores and retrieves
// Blocks and UndoBlocks.
// See config.go for more information on its fields.
// Block files are of the format:
// "DataDirectory/BlockFileName_CurrentBlockFileNumber.FileExtension"
// Ex: "data/block_0.txt"
// UndoBlock files are of the format:
// "DataDirectory/UndoFileName_CurrentUndoFileNumber.FileExtension"
// Ex: "data/undo_0.txt"
type ChainWriter struct {
	// data storage information
	FileExtension string
	DataDirectory string

	// block information
	BlockFileName          string
	CurrentBlockFileNumber uint32
	CurrentBlockOffset     uint32
	MaxBlockFileSize       uint32

	// undo block information
	UndoFileName          string
	CurrentUndoFileNumber uint32
	CurrentUndoOffset     uint32
	MaxUndoFileSize       uint32
}

// New returns a ChainWriter given a Config.
func New(config *Config) *ChainWriter {
	if err := os.MkdirAll(config.DataDirectory, 0700); err != nil {
		log.Fatalf("Could not create ChainWriter's data directory")
	}
	return &ChainWriter{
		FileExtension:          config.FileExtension,
		DataDirectory:          config.DataDirectory,
		BlockFileName:          config.BlockFileName,
		CurrentBlockFileNumber: 0,
		CurrentBlockOffset:     0,
		MaxBlockFileSize:       config.MaxBlockFileSize,
		UndoFileName:           config.UndoFileName,
		CurrentUndoFileNumber:  0,
		CurrentUndoOffset:      0,
		MaxUndoFileSize:        config.MaxUndoFileSize,
	}
}

// StoreBlock stores a Block and its corresponding UndoBlock to Disk,
// returning a BlockRecord that contains information for later retrieval.
func (cw *ChainWriter) StoreBlock(bl *block.Block, undoBlock *UndoBlock, height uint32) *blockinfodatabase.BlockRecord {
	// serialize block
	b := block.EncodeBlock(bl)
	serializedBlock, err := proto.Marshal(b)
	if err != nil {
		utils.Debug.Printf("Failed to marshal block")
	}
	// serialize undo block
	ub := EncodeUndoBlock(undoBlock)
	serializedUndoBlock, err := proto.Marshal(ub)
	if err != nil {
		utils.Debug.Printf("Failed to marshal undo block")
	}
	// write block to disk
	bfi := cw.WriteBlock(serializedBlock)
	// create an empty file info, which we will update if the function is passed an undo block.
	ufi := &FileInfo{}
	if undoBlock.Amounts != nil {
		ufi = cw.WriteUndoBlock(serializedUndoBlock)
	}

	return &blockinfodatabase.BlockRecord{
		Header:               bl.Header,
		Height:               height,
		NumberOfTransactions: uint32(len(bl.Transactions)),
		BlockFile:            bfi.FileName,
		BlockStartOffset:     bfi.StartOffset,
		BlockEndOffset:       bfi.EndOffset,
		UndoFile:             ufi.FileName,
		UndoStartOffset:      ufi.StartOffset,
		UndoEndOffset:        ufi.EndOffset,
	}
}




// WriteBlock writes a serialized Block to Disk and returns a FileInfo for storage information.
func (cw *ChainWriter) WriteBlock(serializedBlock []byte) *FileInfo {
	// check if the current block file needs to be rotated
	if cw.CurrentBlockOffset + uint32(len(serializedBlock)) > cw.MaxBlockFileSize {
		cw.CurrentBlockFileNumber++
		cw.CurrentBlockOffset = 0
	}

	// construct the new block file name
	blockFileName := fmt.Sprintf("%s/%s_%d%s", cw.DataDirectory, cw.BlockFileName, cw.CurrentBlockFileNumber, cw.FileExtension)
	// format a string according to a given format specifier and a list of arguments
	// write the serialized block to disk
	writeToDisk(blockFileName, serializedBlock)

	// create a FileInfo with the start and end offsets of the new block file
	fileInfo := &FileInfo{
		FileName:    blockFileName,
		StartOffset: cw.CurrentBlockOffset,
		EndOffset:   cw.CurrentBlockOffset + uint32(len(serializedBlock)),
	}

	// update the current block file offset
	cw.CurrentBlockOffset += uint32(len(serializedBlock))

	return fileInfo
}


// WriteUndoBlock writes a serialized UndoBlock to Disk and returns
// a FileInfo for storage information.
func (cw *ChainWriter) WriteUndoBlock(serializedUndoBlock []byte) *FileInfo {
	if cw.CurrentUndoOffset + uint32(len(serializedUndoBlock)) > cw.MaxUndoFileSize {
		cw.CurrentUndoFileNumber++
		cw.CurrentUndoOffset = 0
	}

	// construct the new block file name
	blockFileName := fmt.Sprintf("%s/%s_%d%s", cw.DataDirectory, cw.UndoFileName, cw.CurrentUndoFileNumber, cw.FileExtension)
	// format a string according to a given format specifier and a list of arguments
	// write the serialized block to disk
	writeToDisk(blockFileName, serializedUndoBlock)

	// create a FileInfo with the start and end offsets of the new block file
	fileInfo := &FileInfo{
		FileName:    blockFileName,
		StartOffset: cw.CurrentUndoOffset,
		EndOffset:   cw.CurrentUndoOffset + uint32(len(serializedUndoBlock)),
	}

	// update the current block file offset
	cw.CurrentUndoOffset += uint32(len(serializedUndoBlock))

	return fileInfo
}

// ReadBlock returns a Block given a FileInfo.
func (cw *ChainWriter) ReadBlock(fi *FileInfo) *block.Block {
	bytes := readFromDisk(fi)
	pb := &pro.Block{}
	if err := proto.Unmarshal(bytes, pb); err != nil {
		utils.Debug.Printf("failed to unmarshal block from file info {%v}", fi)
	}
	return block.DecodeBlock(pb)
}

// ReadUndoBlock returns an UndoBlock given a FileInfo.
func (cw *ChainWriter) ReadUndoBlock(fi *FileInfo) *UndoBlock {
	bytes := readFromDisk(fi)
	pub := &pro.UndoBlock{}
	if err := proto.Unmarshal(bytes, pub); err != nil {
		utils.Debug.Printf("failed to unmarshal undo block from file info {%v}", fi)
	}
	return DecodeUndoBlock(pub)
}
