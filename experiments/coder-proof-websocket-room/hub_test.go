package main

import (
	"testing"
	"time"
)

func TestHubBroadcast(t *testing.T) {
	hub := newHub()
	go hub.Run()

	client1 := &Client{hub: hub, send: make(chan []byte, 256)}
	client2 := &Client{hub: hub, send: make(chan []byte, 256)}

	hub.register <- client1
	hub.register <- client2

	// Wait for registration to process
	time.Sleep(100 * time.Millisecond)

	payload := []byte("hello world")
	hub.broadcast <- payload

	// Wait for broadcast to process
	time.Sleep(100 * time.Millisecond)

	select {
	case msg := <-client1.send:
		if string(msg) != string(payload) {
			t.Errorf("client1 expected %q, got %q", payload, msg)
		}
	case <-time.After(time.Second):
		t.Fatal("client1 did not receive broadcast")
	}

	select {
	case msg := <-client2.send:
		if string(msg) != string(payload) {
			t.Errorf("client2 expected %q, got %q", payload, msg)
		}
	case <-time.After(time.Second):
		t.Fatal("client2 did not receive broadcast")
	}

	hub.unregister <- client1
	hub.unregister <- client2
}
