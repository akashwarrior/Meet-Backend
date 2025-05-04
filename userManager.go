package main

import (
	"encoding/json"
	"strconv"
	"sync"

	"github.com/gorilla/websocket"
)

type WebRTCEvent string

const (
	OFFER                 WebRTCEvent = "OFFER"
	ANSWER                WebRTCEvent = "ANSWER"
	ICE_CANDIDATE         WebRTCEvent = "ICE_CANDIDATE"
	USER_JOINED           WebRTCEvent = "USER_JOINED"
	USER_LEFT             WebRTCEvent = "USER_LEFT"
	USER_REQUEST          WebRTCEvent = "USER_REQUEST"
	USER_REQUEST_ACCEPTED WebRTCEvent = "USER_REQUEST_ACCEPTED"
	USER_REQUEST_REJECTED WebRTCEvent = "USER_REQUEST_REJECTED"
)

type Message struct {
	Type     WebRTCEvent `json:"type"`
	Sender   string      `json:"sender"`
	Name     string      `json:"name"`
	Receiver string      `json:"receiver"`
	Data     interface{} `json:"data"`
}

type SocketWrapper struct {
	ID   string
	Name string
	Conn *websocket.Conn
}

type UserManager struct {
	HostID       string
	Users        map[string]*SocketWrapper
	WaitingUsers map[string]*SocketWrapper
	GlobalID     int
	mu           sync.Mutex
}

func NewUserManager(hostID string) *UserManager {
	return &UserManager{
		HostID:       hostID,
		Users:        make(map[string]*SocketWrapper),
		WaitingUsers: make(map[string]*SocketWrapper),
		GlobalID:     1,
		mu:           sync.Mutex{},
	}
}

func (um *UserManager) AddUser(sw *SocketWrapper) {
	um.mu.Lock()
	defer um.mu.Unlock()
	if sw.ID == "" {
		sw.ID = strconv.Itoa(um.GlobalID)
		um.GlobalID++
	}

	go um.handleMessages(sw)

	if sw.ID == um.HostID {
		um.joinMeeting(sw)
	} else {
		um.requestToJoin(sw)
	}

	sw.Conn.SetCloseHandler(func(code int, text string) error {
		um.removeUser(sw.ID)
		return nil
	})
}

func (um *UserManager) handleMessages(sw *SocketWrapper) {
	for {
		_, msgBytes, err := sw.Conn.ReadMessage()
		if err != nil {
			um.removeUser(sw.ID)
			return
		}

		var msg Message
		if err := json.Unmarshal(msgBytes, &msg); err != nil {
			continue
		}

		msg.Sender = sw.ID
		msg.Name = sw.Name

		switch msg.Type {
		case OFFER, ANSWER, ICE_CANDIDATE:
			go um.sendMessage(msg)
			break
		case USER_REQUEST:
			go um.requestToJoin(sw)
			break
		case USER_REQUEST_ACCEPTED:
			if user, ok := um.WaitingUsers[msg.Receiver]; ok {
				delete(um.WaitingUsers, user.ID)
				go um.joinMeeting(user)
			}
			break
		case USER_REQUEST_REJECTED:
			um.mu.Lock()
			sw.Conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(4000, "User request rejected"))
			sw.Conn.Close()
			delete(um.WaitingUsers, sw.ID)
			um.mu.Unlock()
			break
		case USER_LEFT:
			go um.removeUser(sw.ID)
			break
		default:
			break
		}
	}
}

func (um *UserManager) requestToJoin(sw *SocketWrapper) {
	um.WaitingUsers[sw.ID] = sw
	msg := Message{
		Type:     USER_REQUEST,
		Sender:   sw.ID,
		Name:     sw.Name,
		Receiver: um.HostID,
	}
	go um.sendMessage(msg)
}

func (um *UserManager) joinMeeting(sw *SocketWrapper) {
	um.Users[sw.ID] = sw
	if sw.ID != um.HostID {
		go um.sendMessage(Message{
			Type:     USER_REQUEST_ACCEPTED,
			Sender:   sw.ID,
			Receiver: sw.ID,
			Name:     sw.Name,
		})
		go um.broadcast(Message{
			Type:   USER_JOINED,
			Sender: sw.ID,
			Name:   sw.Name,
		})
	} else {
		go um.sendMessage(Message{
			Type:     USER_REQUEST_ACCEPTED,
			Sender:   sw.ID,
			Receiver: sw.ID,
			Name:     sw.Name,
		})
	}
}

func (um *UserManager) removeUser(id string) {
	um.mu.Lock()
	defer um.mu.Unlock()

	_, ok := um.Users[id]
	if ok {
		um.Users[id].Conn.Close()
		delete(um.WaitingUsers, id)
	}

	_, ok = um.WaitingUsers[id]
	if ok {
		um.WaitingUsers[id].Conn.Close()
		delete(um.WaitingUsers, id)
	}

	if _, ok := um.Users[id]; ok {
		go um.broadcast(Message{
			Type:   USER_LEFT,
			Sender: id,
		})
		if len(um.Users) == 0 {
			managerMutex.Lock()
			delete(managers, um.HostID)
			managerMutex.Unlock()
		}
	}
}

func (um *UserManager) broadcast(msg Message) {
	for uid, user := range um.Users {
		if uid != msg.Sender {
			um.sendTo(user.Conn, msg)
		}
	}
}

func (um *UserManager) sendMessage(msg Message) {
	if user, ok := um.Users[msg.Receiver]; ok {
		um.sendTo(user.Conn, msg)
	}
}

func (um *UserManager) sendTo(conn *websocket.Conn, msg Message) {
	data, _ := json.Marshal(msg)
	um.mu.Lock()
	defer um.mu.Unlock()
	conn.WriteMessage(websocket.TextMessage, data)
}
