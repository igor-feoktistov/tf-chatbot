package chatbot

import (
	"fmt"
	"time"
	"log"
	"net/http"
	"regexp"
	_ "embed"

	"github.com/gin-gonic/gin"
	"github.com/gin-contrib/sessions"
	"github.com/gorilla/websocket"
)

const (
	EventUserPrompt      = "01"
	EventSystemPrompt    = "02"
	EventAssistantWait   = "03"
	EventAssistantOutput = "04"
	EventAssistantFinish = "05"
	EventPing            = "06"
	EventPong            = "07"
	EventDiagnostic      = "08"
)

//go:embed index.template
var indexHTML string

//go:embed javascript.template
var javascriptJS string

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func handleIndex(c *gin.Context) {
	var chatBot *ChatBot
	if val, exists := c.Get("chatBot"); exists {
		chatBot = val.(*ChatBot)
	} else {
		log.Printf("ERROR: failed to retrieve ChatBot from context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve ChatBot from context"})
		return
	}
	bearerToken, err := c.Cookie("BearerToken")
	if err == nil {
		chatBot.config.ApiKey = bearerToken
	} else {
		log.Printf("WARNING: BearerToken is not found in cookies content")
		if len(chatBot.config.ApiKey) == 0 {
			log.Printf("ERROR: ApiKey is not defined")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "BearerToken is not found in cookies content"})
			return
		}
	}
	c.HTML(http.StatusOK, "index.html", gin.H{
		"Header": "AI ChatBot",
	})
}

func handleJavascript(c *gin.Context) {
	c.HTML(http.StatusOK, "javascript.js", gin.H{})
}

func keepalive(conn *websocket.Conn) {
	ticker := time.NewTicker(60 * time.Second)
	defer func() {
		ticker.Stop()
	}()
	for {
		select {
		case <-ticker.C:
			if err := conn.WriteMessage(websocket.TextMessage, []byte(EventPing + ":ping")); err != nil {
				log.Printf("ERROR: failed to send PING message: %s", err)
				return
			}
		}
	}
}

func handleWebSocket(c *gin.Context) {
	var chatBot *ChatBot
	if val, exists := c.Get("chatBot"); exists {
		chatBot = val.(*ChatBot)
	} else {
		log.Printf("ERROR: failed to retrieve ChatBot from context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve ChatBot from context"})
		return
	}
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("ERROR: failed to upgrade connection to websocket: %s", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upgrade connection to websocket: " + err.Error()})
		return
	}
	go keepalive(conn)
	bearerToken, err := c.Cookie("BearerToken")
	if err == nil {
		chatBot.config.ApiKey = bearerToken
	} else {
		log.Printf("WARNING: BearerToken is not found in cookies content")
		if len(chatBot.config.ApiKey) == 0 {
			log.Printf("ERROR: ApiKey is not defined")
			conn.WriteMessage(websocket.TextMessage, []byte(EventDiagnostic + `:<p style="color: red;"><strong>Websocket error: </strong>BearerToken is not found in cookies content</p>`))
		}
	}
	defer conn.Close()
	for {
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				log.Printf("ERROR: failure while reading websocket message: %s", err.Error())
			} else {
				log.Printf("INFO: websocket connection closed")
			}
			return
		}
		switch messageType {
		case websocket.CloseMessage:
			log.Printf("INFO: peer initiated connection close, closing websocket")
			break
		case websocket.TextMessage, websocket.BinaryMessage:
			re := regexp.MustCompile(`^(\d+):`)
			subMatch := re.FindStringSubmatch(string(message))
			if len(subMatch) != 2 {
				log.Printf("ERROR: received unrecognized websocket message: %s", string(message))
				conn.WriteMessage(messageType, []byte(EventDiagnostic + `:<p style="color: red;"><strong>Websocket error: </strong>` + fmt.Sprintf("received unrecognized websocket message: \"%s\"", string(message)) + `</p>`))
				return
			}
			session := sessions.Default(c)
			switch subMatch[1] {
			case EventPing:
				if err := conn.WriteMessage(messageType, []byte(EventPong + ":pong")); err != nil {
					log.Printf("ERROR: failed to send PONG message: %s", err)
					return
				}
			case EventPong:
				log.Printf("INFO: received PONG reply")
			case EventUserPrompt:
				// Process user prompt
				go func() {
					var err error
					responseChan := chatBot.startCompletionStream(session, string(message[3:]))
					if err = conn.WriteMessage(messageType, []byte(EventAssistantWait)); err == nil {
						for chunk := range responseChan {
							if err = conn.WriteMessage(messageType, []byte(EventAssistantOutput + ":" + chunk)); err != nil {
								break
							}
							if err = conn.WriteMessage(messageType, []byte(EventAssistantWait)); err != nil {
								break
							}
						}
					}
					if err == nil {
						err = conn.WriteMessage(messageType, []byte(EventAssistantFinish))
					}
					if err != nil {
						log.Printf("ERROR: failed to write websocket message: %s", err)
					}
				}()
			case EventSystemPrompt:
				// Save system prompt
				session.Set("systemPrompt", string(message[3:]))
				session.Save()
			default:
				log.Printf("ERROR: received unrecognized websocket event: %s", string(message))
				conn.WriteMessage(messageType, []byte(EventDiagnostic + `:<p style="color: red;"><strong>Websocket error: </strong>` + fmt.Sprintf("received unrecognized websocket event: \"%s\"", string(message)) + `</p>`))
			}
		case websocket.PingMessage:
			log.Printf("INFO: received system PING message", err)
		case websocket.PongMessage:
			log.Printf("INFO: received system PONG message", err)
		default:
			log.Printf("INFO: received unrecognized message type %d", messageType)
		}
	}
}
