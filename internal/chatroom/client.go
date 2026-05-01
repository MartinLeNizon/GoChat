package chatroom

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
)

func StartClient() {
	conn, err := net.Dial("tcp", ":9000")
	if err != nil {
		fmt.Println("Error connecting: ", err)
		return
	}
	defer conn.Close()

	fmt.Println("Connected to chat server")

	go func() {
		reader := bufio.NewReader(conn)
		for {
			message, err := reader.ReadString('\n')
			if err != nil {
				fmt.Println("Disconnected from server.")
				os.Exit(0)
			}
			fmt.Print("\r" + message)
			fmt.Print(">> ")
		}
	}()

	// main go routine: read from stdin
	inputReader := bufio.NewReader(os.Stdin)
	fmt.Println("Welcome to chat server!")

	for {
		fmt.Print(">> ")
		message, _ := inputReader.ReadString('\n')
		message = strings.TrimSpace(message)

		if message == "" {
			continue
		}

		conn.Write([]byte(message + "\n"))
	}

}