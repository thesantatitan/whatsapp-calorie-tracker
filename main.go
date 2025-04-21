package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/mattn/go-sqlite3"
	qrcode "github.com/skip2/go-qrcode"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
)

// renderQR renders the QR code to the terminal
func renderQR(code string) error {
	qr, err := qrcode.New(code, qrcode.Medium)
	if err != nil {
		return err
	}

	// Get the QR code as ASCII art
	art := qr.ToSmallString(false)
	fmt.Println("\nScan this QR code with your WhatsApp app:")
	fmt.Println(art)
	return nil
}

func main() {
	dbLog := waLog.Stdout("Database", "DEBUG", true)
	// Make sure you create the dbFolder
	container, err := sqlstore.New("sqlite3", "file:wastore.db?_foreign_keys=on", dbLog)
	if err != nil {
		panic(err)
	}

	deviceStore, err := container.GetFirstDevice()
	if err != nil {
		panic(err)
	}

	client := whatsmeow.NewClient(deviceStore, waLog.Stdout("Client", "DEBUG", true))

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-ch
		fmt.Println("\nReceived interrupt signal, shutting down...")
		client.Disconnect()
		os.Exit(0)
	}()

	if client.Store.ID == nil {
		// No ID stored, new login required
		qrChan, _ := client.GetQRChannel(context.Background())
		err = client.Connect()
		if err != nil {
			panic(err)
		}
		for evt := range qrChan {
			if evt.Event == "code" {
				// Render the QR code as ASCII art
				err := renderQR(evt.Code)
				if err != nil {
					fmt.Printf("Failed to render QR code: %v\n", err)
					fmt.Println("Raw QR code:", evt.Code)
				}
			} else {
				fmt.Println("Login event:", evt.Event)
			}
		}
	} else {
		// Already logged in, just connect
		err = client.Connect()
		if err != nil {
			panic(err)
		}
		fmt.Println("Successfully logged in!")
	}

	// Wait a moment for connection to establish
	time.Sleep(3 * time.Second)

	// Get recent chats
	fmt.Println("\nFetching recent chats...")
	err = getRecentChats(client)
	if err != nil {
		fmt.Printf("Error getting recent chats: %v\n", err)
	}

	// Keep the program running
	select {}
}

func getRecentChats(client *whatsmeow.Client) error {
	// We'll let the messages handler do the work of collecting recent chats
	recentChats := make(map[string]time.Time)
	maxChats := 5

	// Register the event handler
	client.AddEventHandler(func(evt interface{}) {
		switch v := evt.(type) {
		case *events.Message:
			if len(recentChats) < maxChats {
				recentChats[v.Info.Chat.String()] = v.Info.Timestamp
			}
		}
	})

	fmt.Println("\nMost recent chats (waiting for messages):")
	fmt.Println("------------------")
	fmt.Println("Note: This will populate as new messages arrive")
	fmt.Println("Press Ctrl+C to exit")

	return nil
}
