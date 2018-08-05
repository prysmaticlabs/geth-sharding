// Package p2p handles peer-to-peer networking for Ethereum 2.0 clients.
//
// There are three types of p2p communications.
//
// 	- Direct: two peer communication
// 	- Floodsub: peer broadcasting to all peers
// 	- Gossipsub: peer broadcasting to localized peers
//
// Read more about gossipsub at https://github.com/vyzo/gerbil-simsub
//
// Notes:
// Gossip sub topics can be identified by their proto message types.
//
// 		topic := proto.MessageName(myMsg)
//
// Then we can assume that only these message types are broadcast in that
// gossip subscription.
package p2p

import "context"

// Use this file for interfaces only!

// Adapters are used to create middleware.
type Adapter func(context.Context, Message, Handler)

type Handler func(context.Context, Message)
