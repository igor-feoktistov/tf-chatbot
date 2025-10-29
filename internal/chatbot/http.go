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
	EventUserPrompt   = "01"
	EventSystemPrompt = "02"
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
	accessToken, err := c.Cookie("BearerToken")
	if err == nil {
		chatBot.config.ApiKey = accessToken
	} else {
		log.Printf("WARNING: accessToken not found in cookies content")
		if len(chatBot.config.ApiKey) == 0 {
			log.Printf("Failed to retrieve ChatBot from context")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "accessToken not found in cookies content"})
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
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
	}()
	for {
		select {
		case <-ticker.C:
			if err := conn.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				log.Printf("Error sending ping: %s", err.Error())
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
	go keepalive(conn)
	if err != nil {
		log.Printf("ERROR: failed to upgrade connection to websocket: %s", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upgrade connection to websocket: " + err.Error()})
		return
	}
	defer conn.Close()
	for {
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			log.Printf("ERROR: failed to read websocket message: %s", err)
			return
		}
		switch messageType {
		case 8:
			log.Printf("INFO: Peer initiated connection close, closing websocket")
			break
		case 1, 2:
			re := regexp.MustCompile(`^(\d+):`)
			subMatch := re.FindStringSubmatch(string(message))
			if len(subMatch) != 2 {
				log.Printf("ERROR: received unrecognized websocket message: %s", string(message))
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Received unrecognized websocket message: " + string(message)})
				return
			}
			session := sessions.Default(c)
			switch subMatch[1] {
			case EventUserPrompt:
				// Process user prompt
				go func() {
					responseChan := chatBot.startCompletionStream(session, string(message[3:]))
					for chunk := range responseChan {
						if err := conn.WriteMessage(messageType, []byte(chunk)); err != nil {
							log.Printf("ERROR: failed to write websocket message: %s", err)
							return
						}
					}
				}()
			case EventSystemPrompt:
				// Save system prompt
				session.Set("systemPrompt", string(message[3:]))
				session.Save()
			default:
				log.Printf("ERROR: received unrecognized websocket event: %s", string(message))
				conn.WriteMessage(messageType, []byte(fmt.Sprintf("ERROR: received unrecognized websocket event: \"%s\"", string(message))))
			}
		case 9:
			log.Printf("INFO: received PING message type", err)
		case 10:
			log.Printf("INFO: received PONG message type", err)
		default:
			log.Printf("INFO: received unrecognized message type %d", messageType)
		}
	}
}
