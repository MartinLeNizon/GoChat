package chatroom

import (
	"net"
	"os"
	"sync"
	"time"
)

type Message struct {
	ID			int			`json:"id"`	
	From 		string		`json:"from"`
	Content		string		`json:"content"`
	Timestamp 	time.Time	`json:"timestamp"`
	Channel		string		`json:"channel"`
}

type Client struct {
	conn			net.Conn
	username		string
	outgoing		chan string
	lastActive		time.Time
	messagesSent 	int
	messagesRecv	int
	isSlowClient	bool
	reconnectToken	string
	mu				sync.Mutex
}

type Chatroom struct {
	// Communication channels
	join			chan *Client
	leave			chan *Client
	broadcast		chan string
	listUsers		chan *Client
	directMessage	chan DirectMessage

	// State
	clients 		map[*Client]bool
	mu 				sync.Mutex
	totalMessages	int
	startTime		time.Time

	// Message history
	messages		[]Message
	messageMu		sync.Mutex
	nextMessageID	int

	// Persistence
	walFile			*os.File
	walMu			sync.Mutex
	dataDir			string

	// Sessions
	sessions		map[string]*SessionInfo
	sessionsMu		sync.Mutex
}

type SessionInfo struct {
	Username		string
	ReconnectToken	string
	LastSeen		time.Time
	CreatedAt		time.Time
}

type DirectMessage struct {
	toClient 	*Client
	message		string
}