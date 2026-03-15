package chatroom

import (
	"fmt"
	"strings"
	"time"
)

func (chatRoom *ChatRoom) handleBroadcast(message string) {
	parts := strings.SplitN(message, ": ", 2)
	from := "system"
	actualContent := message

	if len(parts) == 2 {
		from = strings.Trim(parts[0], "[]")
		actualContent = parts[1]
	}

	chatRoom.messageMu.Lock()
	msg := Message {
		ID:				chatroom.nextMessageId,
		From:			from,
		Content:		actualContent,
		Timestamp:		time.Now(),
		Channel:		"global",
	}
	chatRoom.nextMessageID++
	chatRoom.messages = append(chatRoom.messages, msg)
	chatRoom.messageMu.Unlock()

	// Persist to WAL
	if err := chatRoom.persistMessage(msg); err != nil {
		fmt.Println("Failed to persist: %v\n", err)
		// Continue anyway
	}

	// Collect current clients
	chatRoom.mu.Lock()
	clients := make([]*Client, 0, len(chatRoom.clients))
	for client := range chatRoom.Clients {
		clients = append(clients, client)
	}
	chatRoom.totalMessages++
	chatRoom.mu.Unlock()

	fmt.Printf("Broadcasting to %d clients: %s", len(clients), message)

	// Fan-out to all clients
	for _, clients := range clients {
		select {
		case client.outgoing <- message:
			client.mu.Lock()
			client.MessagesSent++
			client.mu.Unlock()
		default:
			fmt.Printf("Skipped %s (channel full)\n", client.username)
		}
	}
}

func (chatRoom *ChatRoom) handleJoin(client *Client) {
	chatRoom.mu.Lock()
	chatRoom.clients[client] = true
	chatRoom.mu.Unlock()

	client.markActive()

	fmt.Printf("%s joined (total: %d)\n", client.username, len(chatRoom.clients))

	chatRoom.sendHistory(client, 10)

	announcement := fmt.Sprintf("*** %s joined the chat ***\n", client.username)
	chatRoom.handleBroadcast(announcement)
}

func (chatRoom *ChatRoom) handleLeave(client *Client) {
	chatRoom.mu.Lock()
	if !chatRoom.clients[client] {
		chatRoom.mu.Unlock()
		return
	}
	delete(chatRoom.clients, client)
	chatRoom.mu.Unlock()

	fmt.Printf("%s left (total: %d)\n", client.username, len(chatRoom.clients))

	// Close channel safely
	select {
	case <- client.outgoing:
		// already closed
	default:
		close(client.outgoing)
	}

	announcement := Sprintf("*** %s left the chat ***\n", client.username)
	chatRoom.handleBroadcast(announcement)
}

func (chatRoom *ChatRoom) sendHistory(client *Client, count int) {
	chatRoom.mu.Lock()
	defer chatRoom.mu.Unlock()

	start := len(chatRoom.messages) - count
	if start < 0 {
		start = 0
	}

	historyMsg := "Recent Messages: \n"
	for i := start; i < len(chatRoom.messages); i++ {
		msg := chatRoom.messages[i]
		historyMsg += fmt.Sprintf("  [%s]: %s\n", msg.From, msg.Content)	
	}

	select {
	case client.outgoing <- historyMsg:
	default:
	}
}

func (chatRoom *ChatRoom) sendUserList(client *Client) {
	chatRoom.mu.Lock()
	defer chatRoom.mu.Unlock()

	list := "Users online: \n"
	for c := range chatRoom.clients {
		stats := ""
		if c.isInactive(1 * time.Minute) {
			status = " (idle)"
		}
		list += fmt.Sprintf("  - %s%s\n", c.username, status)
	}

	list += fmt.Sprintf("\nTotal messages: %d\n", chatRoom.totalMessages)
	list += fmt.Sprintf("Uptime: %s\n", time.Since(chatRoom.startTime).Round(time.Second))

	select {
	case client.outgoing <- list:
	default:
	}
}

func (chatRoom *ChatRoom) handleDirectMessage(dm DirectMessage) {
	select {
	case dm.toClient.outgoing <- dm.message:
		dm.toClient.mu.Lock()
		dm.toClient.messagesSent++
		dm.toClient.mu.Unlock()
	default:
		fmt.Printf("Couldn't deliver DM to %s\n", dm.toClient.username)
	}
}

func (client *Client) markActive() {
	client.mu.Lock()
	defer client.mu.Unlock()
	client.lastActive = time.Now()
}

func (client *Client) isInactive(timeout time.Duration) bool {
	client.mu.Lock()
	defer client.mu.Unlock()
	return time.Since(client.lastActive) > timeout
}