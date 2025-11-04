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
	"github.com/openai/openai-go/v3"
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
	EventConfirmed       = "09"
	EventResetHistory    = "10"
	EventEnableHistory   = "11"
	EventDisableHistory  = "12"
)

//go:embed index.template
var indexHTML string

//go:embed javascript.template
var javascriptJS string

//go:embed styles.template
var stylesCSS string

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
		"Header": "ChatBot",
	})
}

func handleJavascript(c *gin.Context) {
	c.HTML(http.StatusOK, "javascript.js", gin.H{})
}

func handleStyles(c *gin.Context) {
	c.Header("Content-Type", "text/css")
	c.HTML(http.StatusOK, "styles.css", gin.H{})
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
	resetHistory := true
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
			var err error
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
					responseChan := chatBot.startCompletionStream(session, string(message[3:]), resetHistory)
					for chunk := range responseChan {
						if err = conn.WriteMessage(messageType, []byte(EventAssistantOutput + ":" + chunk)); err != nil {
							break
						}
						if err = conn.WriteMessage(messageType, []byte(EventAssistantWait)); err != nil {
							break
						}
					}
					if err == nil {
						err = conn.WriteMessage(messageType, []byte(EventAssistantFinish))
					}
					resetHistory = false
				}()
				err = conn.WriteMessage(messageType, []byte(EventConfirmed + ":" + EventUserPrompt))
			case EventSystemPrompt:
				// Save system prompt
				session.Set("systemPrompt", string(message[3:]))
				messages := []openai.ChatCompletionMessageParamUnion{}
				session.Set("messages", messages)
				session.Save()
				err = conn.WriteMessage(messageType, []byte(EventConfirmed + ":" + EventSystemPrompt))
			case EventResetHistory:
				// Reset history
				if !resetHistory {
					log.Printf("INFO: received EventResetHistory")
					if err = conn.WriteMessage(messageType, []byte(EventConfirmed + ":" + EventResetHistory)); err == nil {
						messages := []openai.ChatCompletionMessageParamUnion{}
						session.Set("messages", messages)
						session.Save()
						resetHistory = true
					}
				}
			case EventDisableHistory:
				// Disable history
				log.Printf("INFO: received EventDisableHistory")
				chatBot.config.ChatOptions.ChatHistory = false
				err = conn.WriteMessage(messageType, []byte(EventConfirmed + ":" + EventDisableHistory))
			case EventEnableHistory:
				// Enable history
				log.Printf("INFO: received EventEnableHistory")
				chatBot.config.ChatOptions.ChatHistory = true
				err = conn.WriteMessage(messageType, []byte(EventConfirmed + ":" + EventEnableHistory))
			default:
				log.Printf("ERROR: received unrecognized websocket event: %s", string(message))
				conn.WriteMessage(messageType, []byte(EventDiagnostic + `:<p style="color: red;"><strong>Websocket error: </strong>` + fmt.Sprintf("received unrecognized websocket event: \"%s\"", string(message)) + `</p>`))
			}
			if err != nil {
				log.Printf("ERROR: websocket error: %s", err)
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
