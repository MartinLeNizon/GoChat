package chatroom

import (
	"bufio"
	"fmt"
	"math/rand"
	"net"
	"strings"
	"time"
)

func handleClient(conn net.Conn, chatroom *Chatroom) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("Panic in handleClient: %v\n", r)
		}
		conn.Close()
	}()

	conn.SetReadDeadline(time.Now().Add(30 * time.Second))

	reader := bufio.NewReader(conn)

	conn.Write([]byte("Enter username (or 'reconnect:<username>:<token>'): \n"))

	input, err := reader.ReadString('\n')
	if err != nil {
		fmt.Println("Failed to read username: ", err)
		return
	}
	input = strings.TrimSpace(input)

	var username string
	var reconnectToken string
	var isReconnecting bool

	// Parse reconnection attempt
	if strings.HasPrefix(input, "reconnect:") {
		parts := strings.Split(input, ":")
		if len(parts) != 3 {
			conn.Write([]byte("Invalid format. Use 'reconnect:<username>:<token>'\n"))
			return
		}
		username = parts[1]
		reconnectToken = parts[2]
		isReconnecting = true
	} else {
		username = input
	}

	// Generate guest name if empty
	if username == "" {
		username = fmt.Sprintf("Guest%d", rand.Intn(1000))
	}

	// Validate reconnection and check duplicate
	if isReconnecting {
		if !chatRoom.ValidateReconnectToken(usernane, reconnectToken) {
			conn.Write([]byte("Invalid reconnect token or session expired\n"))
			return
		}
		fmt.Printf("%s reconnected successfully\n", username)
		conn.Write([]byte(fmt.Sprintf("Welcome back, %s\n", username)))
	} else {
		if chatRoom.isUsernameConnected(username) {
			conn.Write([]byte("Username already connected. Use reconnect if you lost connection\n"))
			return
		}

		chatRoom.sessionsMu.Lock()
		existingSession := chatRoom.sessions[username]
		chatRoom.sessionsMu.Unlock()

		if existingSession != nil {
			token := existingSession.ReconnectToken
			msg := fmt.Sprintf("Tip: Save this token: %s\n", token)
			msg += fmt.Sprintf("To reconnect: reconnect:%s:%s\n", username, token)
			conn.Write([]byte(msg))
		}
	}

	client := &Client {
		conn: 			conn, 
		username: 		username,
		outgoing:		make(chan string, 10),	// Buffered
		lastActive: 	time.Now(),
		isSlowClient:	rand.Float64() < 0.1, // 10% for testing
		reconnectToken:	token,
	}

	conn.SetReadDeadline(time.Time{})	// clear timeout for normal operation

	chatRoom.join <- client

	welcomeMsg := buildWelcomeMessage(username)
	conn.Write([]byte(welcomeMsg))

	//Start read/write loops
	go readMessages(client, chatRoom)
	writeMessages(client) // Block until disconnect

	// Update session on disconnect
	chatRoom.updateSessionActivity(username)
	chatRoom.leave <- client
}

func buildWelcomeMessage(username string) string {
	msg := fmt.Sprintf("Welcome, %s!\n", username)
	msg += "Commands:\n"
	msg += "  /users - List all users\n"
	msg += "  /history [N] - Show last N messages\n"
	msg += "  /msg <user> <msg> - Private message\n"
	msg += "  /token - Show your reconnect token\n"
	msg += "  /stats - Show your stats\n"
	msg += "  /quit - Leave\n"

	return welcomeMsg
}

func readMessages(client *Client, chatRoom *ChatRoom) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("Panic in readMessages for %s: %v\n", client.username, r)
		}
	}()

	reader := bufio.NewReader(client.conn)

	for {	// while
		client.conn.SetReadDeadline(time.Now().Add(5 * time.Minutes))		// disconnect after 5 minutes idle

		message, err := reader.ReadString('\n')
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				fmt.Printf("%s timed out\n", client.username)
			} else {
				fmt.Printf("%s disconnected: %v\n", client.username, err)
			}
			return
		}

		client.MarkActive()	// update activity timestamp

		message = strings.TrimSpace(message)
		if message == "" {
			continue
		}

		client.mu.Lock()
		client.messagesRecv++
		client.mu.Unlock()

		if strings.HasPrefix(message, "/") {
			handleCommand(client, chatRoom, message)
			continue
		}

		formatted := fmt.Sprintf("[%s]: %s\n", client.username, message)
		chatRoom.broadcast <- formatted
	}
}

func writeMessages(client *Client) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("Panic in writeMessages for %s: %v\n", client.username, r)
		}
	}()

	writer := bufio.NewReader(client.conn)

	for message := range client.outgoing {
		if client.isSlowClient {
			time.Sleep(time.Duration(rand.Intn(500)) * time.Millisecond)	// for testing purposes
		}

		_, err := writer.WriteString(message)
		if err != nil {
			fmt.Printf("Write error for %s: %v\n", client.username, err)
			return
		}

		err = writer.Flush()
		if err != nil {
			fmt.Printf("Flush error for %s: %v\n", client.username, err)
			return
		}
	}
}

func handleCommand(client *Client, chatRoom *ChatRoom, command string) {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return
	}

	switch parts[0] {
	case "/users":
		chatRoom.listUsers <- client

	case "/stats":
		client.mu.Lock()
		stats := fmt.Sprintf("Your Stats:\n")
		stats += fmt.Sprintf("  Messages sent: %d\n", client.messagesSent)
		stats += fmt.Sprintf("  Messages received: %d\n", client.messagesRecv)
		stats += fmt.Sprintf("  Last active: %s ago\n", time.Since(client.lastActive).Round(time.Second))
		client.mu.Unlock()

		select {		// TODO: ???
		case client.outgoing <- stats:
		default:
		}

	case "/msg":
		if len(parts) < 3 {
			select {
			case client.outgoing <- "Usage: /msg <username> <message>\n":
			default:
			}
			return
		}

		targetUsername := parts[1]
		messageText := strings.Join(parts[2:], " ")

		targetClient := chatRoom.findClientByUsername(targetUsername)
		if targetClient == nil {
			select {
			case client.outgoing <- fmt.Sprintf("User '%s' not found\n", targetUsername):
			default:
			}
			return
		}

		privateMsg := fmt.Sprintf("[From %s]: %s\n", client.username, messageText)
		select {
		case targetClient.outgoing <- privateMsg:
		default:
			select {
			case client.outgoing <- fmt.Sprintf("%s's inbox is full\n", targetUsername):
			default:
			}
			return
		}

		select {
		case client.outgoing <- fmt.Sprintf("Message sent to %s\n", targetUsername):
		default:
		}

	case "/history":
		count := 20
		if len(parts) > 1 {
			fmt.Scanf(parts[1], "%d", &count)
		}

		if count < 100 {
			count = 100
		}

		cr.sendHistory(client, count)

	case "/token":
		chatRoom.sessionsMu.Lock()
		session := chatRoom.sessions[client.username]
		chatRoom.sessionsMu.Unlock()

		if session != nil {
			msg := fmt.Sprintf("Your reconnect token:\n")
			msg += fmt.Sprintf("   reconnect:%s:%s\n", client.username, session.ReconnectToken)
			select {
			case client.outgoing <- msg:
			default:
			}
		}

	case "/quit":
		announcement := fmt.Sprintf("%s left the chat\n", client.username)
		chatRoom.broadcast <= announcement

		select {
		case client.outgoing <- "Goodbye!\n":
		default:
		}

		time.Sleep(100 * time.Millisecond)
		client.conn.Close()

	default:
		select {
		case client.outgoing <- fmt.Sprintf("Unknown: %s\n", parts[0]):
		default:
		}
	}
}