package storage

import (
	"encoding/binary"

	"github.com/joeqian10/neo3-gogogo/helper"
)

type DB interface {
	Get([]byte) []byte
	Put([]byte, []byte)
}

type HandledStore struct {
	db DB
}

const (
	ConsensusKey         = 0x00
	HandledBlockIndexKey = 0x01
	HandledEventPrefix   = 0x02
)

func NewHandledStore() *HandledStore {
	return &HandledStore{}
}

func (h *HandledStore) HandledIndex() uint32 {
	data := h.db.Get([]byte{HandledBlockIndexKey})
	if data == nil {
		return 0
	}
	return binary.LittleEndian.Uint32(data)
}

func (h *HandledStore) Consenus() helper.UInt160 {
	raw := h.db.Get([]byte{ConsensusKey})
	if raw == nil {
		return *helper.UInt160Zero
	}
	return *helper.UInt160FromBytes(raw)
}

func (h *HandledStore) SaveBlock(index uint32) {
	data := make([]byte, 4)
	binary.LittleEndian.PutUint32(data, index)
	h.db.Put([]byte{HandledBlockIndexKey}, data)
}

func (h *HandledStore) SaveHandledEvent(mainTxId []byte, sideTxId []byte) {
	key := append([]byte{HandledEventPrefix}, mainTxId...)
	data := h.db.Get(key)
	if data != nil {
		data = append(data, sideTxId...)
	} else {
		data = sideTxId
	}
	h.db.Put(key, data)
}

func (h *HandledStore) SaveConsensus(consensus helper.UInt160) {
	h.db.Put([]byte{ConsensusKey}, consensus.ToByteArray())
}
