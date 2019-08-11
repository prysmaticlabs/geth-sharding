package hobbits

import (
	"context"
	"fmt"
	"time"

	"github.com/gogo/protobuf/proto"
	peer "github.com/libp2p/go-libp2p-peer"
	"github.com/pkg/errors"
	"github.com/prysmaticlabs/go-ssz"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	v1alpha1 "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/shared/hashutil"
	"github.com/prysmaticlabs/prysm/shared/p2p"
	"github.com/renaynay/go-hobbits/encoding"
	log "github.com/sirupsen/logrus"
	"gopkg.in/mgo.v2/bson"
)

type RPCHeader struct {
	MethodID uint16 `bson:"method_id"`
	Id       uint64 `bson:"id"`
}

type Hello struct {
	NodeID               string   `bson:"node_id"`
	LatestFinalizedRoot  [32]byte `bson:"latest_finalized_root"`
	LatestFinalizedEpoch uint64   `bson:"latest_finalized_epoch"`
	BestRoot             [32]byte `bson:"best_root"`
	BestSlot             uint64   `bson:"best_slot"`
}

type GossipHeader struct {
	MethodID    uint16   `bson:"method_id"`
	Topic       string   `bson:"topic"`
	Timestamp   uint64   `bson:"timestamp"`
	MessageHash [32]byte `bson:"message_hash"`
	Hash        [32]byte `bson:"hash"`
}

type Status struct {
	UserAgent []byte `bson:"user_agent"`
	Timestamp uint64 `bson:"timestamp"`
}

type BlockBodiesRequest struct {
	StartRoot []byte `bson:"start_root"`
	StartSlot uint64 `bson:"start_slot"`
	Max       uint64 `bson:"max"`
	Skip      uint64 `bson:"skip"`
	Direction uint8  `bson:"direction"`
}

type BlockBodiesResponse struct {
	Bodies []*v1alpha1.BeaconBlockBody `bson:"bodies"`
}

type BlockBodyResponse struct {
	Bodies *v1alpha1.BeaconBlock `bson:"bodies"`
}

type AttestationRequest struct {
	Hash []byte `bson:"hash"`
}

type AttestationResponse struct {
	Attestation v1alpha1.Attestation `bson:"attestation"`
}

func (h *HobbitsNode) status(id peer.ID, message HobbitsMessage) error {
	// TODO does something with the status of the other node

	responseBody := Status{
		UserAgent: []byte(fmt.Sprintf("prysm node %s", h.NodeId)),
		Timestamp: uint64(time.Now().Unix()),
	}

	body, err := bson.Marshal(responseBody)
	if err != nil {
		return errors.Wrap(err, "error marshaling response body")
	}

	responseMessage := HobbitsMessage{
		Version:  message.Version,
		Protocol: message.Protocol,
		Header:   message.Header,
		Body:     body,
	}

	err = h.Server.SendMessage(h.PeerConns[id], encoding.Message(responseMessage))
	if err != nil {
		return errors.Wrap(err, "error sending GET_STATUS")
	}

	return nil
}

func (h *HobbitsNode) sendHello(id peer.ID, message HobbitsMessage) error {
	response := h.rpcHello()

	responseBody, err := bson.Marshal(response)

	responseMessage := HobbitsMessage{
		Version:  message.Version,
		Protocol: message.Protocol,
		Header:   message.Header,
		Body:     responseBody,
	}
	log.Trace(responseMessage)

	err = h.Server.SendMessage(h.PeerConns[id], encoding.Message(responseMessage))
	if err != nil {
		return errors.Wrap(err, "error sending HELLO")
	}

	log.Trace("sending HELLO...")
	return nil
}

func (h *HobbitsNode) rpcHello() Hello {
	var response Hello

	response.NodeID = h.NodeId
	response.BestRoot = h.DB.HeadStateRoot()

	headState, err := h.DB.HeadState(context.Background())
	if err != nil {
		log.Printf("error getting HeadState data from db: %s", err.Error())
	} else {
		response.BestSlot = headState.Slot // best slot
	}

	finalizedState, err := h.DB.FinalizedState()
	if err != nil {
		log.Printf("error getting FinalizedState data from db: %s", err.Error())
	} else {
		response.LatestFinalizedEpoch = finalizedState.Slot / 64 // finalized epoch

		hashedFinalizedState, err := hashutil.HashProto(finalizedState) // finalized root
		if err != nil {
			log.Printf("error hashing FinalizedState: %s", err.Error())
		} else {
			response.LatestFinalizedRoot = hashedFinalizedState
		}
	}

	return response
}

func (h *HobbitsNode) removePeer(id peer.ID) error {
	peerToRemove := h.PeerConns[id]
	delete(h.PeerConns, id)

	err := peerToRemove.Close()
	if err != nil {
		return errors.Wrap(err, "error closing connection on peerToRemove")
	}

	return nil
}

func (h *HobbitsNode) blockHeadersRequest(id peer.ID, message HobbitsMessage) error { // TODO all block header funcs
	return nil
}

func (h *HobbitsNode) blockHeadersResponse(msg proto.Message) (HobbitsMessage, error) {
	return HobbitsMessage{}, nil
}

func (h *HobbitsNode) receivedBlockHeaders(message HobbitsMessage) error {
	var header *v1alpha1.BeaconBlockHeader

	err := ssz.Unmarshal(message.Body, header)
	if err != nil {
		return errors.Wrap(err, "could not unmarshal block headers")
	}

	return nil
}

func (h *HobbitsNode) blockBodyRequest(id peer.ID, message HobbitsMessage) error {
	var requestBody BlockBodiesRequest
	err := bson.Unmarshal(message.Body, requestBody)
	if err != nil {
		return errors.Wrap(err, "could not unmarshal body of GET_BLOCK_BODY request")
	}

	bbr := pb.BeaconBlockRequest{
		Hash: requestBody.StartRoot,
	}

	h.Feed(&bbr).Send(p2p.Message{
		Ctx:  context.Background(),
		Data: &bbr,
		Peer: id,
	})

	return nil
}

func (h *HobbitsNode) blockBodiesResponse(msg proto.Message) (HobbitsMessage, error) {
	blockBody := BlockBodyResponse{
		Bodies: msg.(*pb.BeaconBlockResponse).Block,
	}
	body, err := bson.Marshal(blockBody)
	if err != nil {
		return HobbitsMessage{}, errors.Wrap(err, "error marshaling body for BLOCK_BODIES response")
	}

	head := RPCHeader{
		MethodID: BLOCK_BODIES,
	}
	header, err := bson.Marshal(head)
	if err != nil {
		return HobbitsMessage{}, errors.Wrap(err, "error marshaling header for BLOCK_BODIES response")
	}

	return HobbitsMessage{
		Version:  CurrentHobbits,
		Protocol: encoding.RPC,
		Header:   header,
		Body:     body,
	}, nil
}

func (h *HobbitsNode) receivedBlockBodies(message HobbitsMessage) error {
	var blockBody *v1alpha1.BeaconBlock

	err := ssz.Unmarshal(message.Body, blockBody)
	if err != nil {
		return errors.Wrap(err, "could not unmarshal block body")
	}

	h.Feed(blockBody)

	return nil
}

func (h *HobbitsNode) attestationRequest(id peer.ID, message HobbitsMessage) error {
	var requestBody AttestationRequest

	err := bson.Unmarshal(message.Body, requestBody)
	if err != nil {
		return errors.Wrap(err, "error unmarshaling body of GET_ATTESTATION request")
	}

	ar := &pb.AttestationRequest{
		Hash: requestBody.Hash,
	}

	h.Feed(ar).Send(p2p.Message{
		Ctx:  context.Background(),
		Data: ar,
		Peer: id,
	})

	return nil
}

func (h *HobbitsNode) attestationResponse(msg proto.Message) (HobbitsMessage, error) {
	response := AttestationResponse{
		Attestation: *msg.(*pb.AttestationResponse).Attestation,
	}
	body, err := bson.Marshal(response)
	if err != nil {
		return HobbitsMessage{}, errors.Wrap(err, "error marshaling body for ATTESTATION response")
	}

	head := RPCHeader{
		MethodID: ATTESTATION,
	}
	header, err := bson.Marshal(head)
	if err != nil {
		return HobbitsMessage{}, errors.Wrap(err, "error marshaling header for ATTESTATION response")
	}

	return HobbitsMessage{
		Version:  CurrentHobbits,
		Protocol: encoding.RPC,
		Header:   header,
		Body:     body,
	}, nil
}

func (h *HobbitsNode) receivedAttestation(message HobbitsMessage) error {
	var attestation *v1alpha1.Attestation

	err := ssz.Unmarshal(message.Body, attestation)
	if err != nil {
		return errors.Wrap(err, "could not unmarshal attestation")
	}

	h.Feed(attestation)

	return nil
}
