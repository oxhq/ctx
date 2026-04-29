package state

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"

	"github.com/oxhq/ctx/internal/compiler"
	"github.com/oxhq/ctx/internal/jsonutil"
	"github.com/oxhq/ctx/internal/store"
)

type Ref string

type Delta struct {
	Add    []compiler.ContextItem `json:"add"`
	Remove []string               `json:"remove"`
}

type Store struct {
	db *store.DB
}

func New(db *store.DB) Store {
	return Store{db: db}
}

func (s Store) Put(packet compiler.ContextPacket) (Ref, error) {
	body, err := jsonutil.MarshalStable(packet)
	if err != nil {
		return "", err
	}
	ref := refFor(body)
	packet.Meta.ContextRef = string(ref)
	body, err = jsonutil.MarshalStable(packet)
	if err != nil {
		return "", err
	}
	ref = refFor(body)
	packet.Meta.ContextRef = string(ref)
	body, err = jsonutil.MarshalStable(packet)
	if err != nil {
		return "", err
	}
	return ref, s.db.PutState(string(ref), body)
}

func (s Store) Get(ref Ref) (compiler.ContextPacket, error) {
	body, err := s.db.GetState(string(ref))
	if err != nil {
		return compiler.ContextPacket{}, err
	}
	var packet compiler.ContextPacket
	if err := json.Unmarshal(body, &packet); err != nil {
		return compiler.ContextPacket{}, err
	}
	return packet, nil
}

func (s Store) ApplyDelta(ref Ref, delta Delta) (Ref, error) {
	packet, err := s.Get(ref)
	if err != nil {
		return "", err
	}
	remove := map[string]bool{}
	for _, id := range delta.Remove {
		remove[id] = true
	}
	var next []compiler.ContextItem
	for _, item := range packet.Context {
		if !remove[item.ID] {
			next = append(next, item)
		}
	}
	next = append(next, delta.Add...)
	sort.Slice(next, func(i, j int) bool { return next[i].ID < next[j].ID })
	packet.Context = next
	return s.Put(packet)
}

func refFor(body []byte) Ref {
	sum := sha256.Sum256(body)
	return Ref("session_" + hex.EncodeToString(sum[:])[:16])
}
