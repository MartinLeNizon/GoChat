package chatroom

import (
	"fmt"
	"net"
	"time"
)

func NewChatRoom(dataDir string) (*ChatRoom, error) {
	cr := &ChatRoom{
		clients:		make(map[*Client]bool),
		join:			make(chan *Client),
		leave:			make(chan *Client),
		broadcast:		make(chan string),
		listUsers:		make(chan *Client),
		directMessage:	make(chan DirectMessage),

		sessions:		make(map[string]*SessionInfo),
		messages:		make([]Message, 0),
		startTime:		time.Now(),
		dataDir:		dataDir,
	}

	if err := cr.loadSnapshot(); err != nil {
		fmt.Printf("Failed to load snapshot: %v\n", err)
	}

	if err := cr.initializePersistence(); err != nil {
		return nil, err
	}

	go cr.periodicSnapshots()

	return cr, nil
}

func (cr *ChatRoom) periodicSnapshots() {
	ticker := time.NewTicker(5 * time.Minutes)			// TODO: Make global
	defer ticker.Stop()

	for range ticker.C {
		cr.messageMu.Lock()
		messageCount := len(cr.messages)
		cr.messageMu.Unlock()

		if (messageCount > 100) {		// TODO: Make global
			if err := cr.createSnapshot(); err != nil {
				fmt.Printf("Snapshot failed: %v\n", err)
			}
		}
	}
}

func (cr *ChatRoom) Run() {
	fmt.Println("GoChat heartbeat started...")
	go cr.cleanupInactiveClients()			// TODO: IMPLEMENT

	for {
		select {
		case client := <-cr.join:
			cr.handleJoin(client)				// TODO: IMPLEMENT
		case client:= <-cr.leave:
			cr.handleLeave(client)				// TODO: IMPLEMENT
		case message := <-cr.broadcast:
			cr.handleBroadcast(message)			// TODO: IMPLEMENT
		case client := <-cr.listUsers:
			cr.sendUserList(client)				// TODO: IMPLEMENT
		case dm := <-cr.directMessage:
			cr.handleDirectMessage(dm)			// TODO: IMPLEMENT
		}
	}
}


import (
	"os"
	"os/signal"
	"syscall"
)

func runServer() {
	chatRoom, err := NewChatRoom(dataDir: "./chatdata")
	if err != nil {
		fmt.Printf("Failed to initialize: %v\n", err)
		return
	}
	defer chatRoom shutdown()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\nReceived shutdown signal")
		chatRoom.shutdown()
		os.Exit(0)
	}()

	go chatRoom.Run()

	listener, err := net.Listen("tcp", ":9000")			 // TODO: make global
	if err != nil {
		fmt.Println("Error starting server:", err)
		return
	}
	defer listener.Close()

	fmt.Printf("Server started on :9000")		// TODO: make global

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Error accepting connection", err)
			continue
		}
		fmt.Println("New Connection from: ", conn.RemoteAddr())
		go handleClient(conn, chatRoom)			// TODO: impl
	}
}

func (cr *ChatRoom) shutdown() {
	fmt.Println("\nShutting down...")
	if err := cr.CreateSnapshot(); err != nil {
		fmt.Printf("Final snapshot failed: %v\n", err)
	}
	if cr.walFile != nil {
		cr.walFile.Close()
	}
	fmt.Println("Shutdown complete")
}