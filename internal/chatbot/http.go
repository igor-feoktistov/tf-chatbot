package chatbot

import (
	"fmt"
	"time"
	"log"
	"context"
	"net/http"
	"regexp"
	_ "embed"

	"github.com/gin-gonic/gin"
	"github.com/gin-contrib/sessions"
	"github.com/gorilla/websocket"
	"github.com/openai/openai-go/v3"
)

const (
	EventUserPrompt       = "01"
	EventSystemPrompt     = "02"
	EventAssistantWait    = "03"
	EventAssistantOutput  = "04"
	EventAssistantFinish  = "05"
	EventPing             = "06"
	EventPong             = "07"
	EventDiagnostic       = "08"
	EventConfirmed        = "09"
	EventResetHistory     = "10"
	EventEnableHistory    = "11"
	EventDisableHistory   = "12"
	EventCancelUserPrompt = "14"
	EventLoadSystemPrompt = "15"
)

//go:embed index.template
var indexHTML string

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func handleIndex(c *gin.Context) {
	var chatBot *ChatBot
	logger := log.New(gin.DefaultWriter, "[CHATBOT] ", log.LstdFlags)
	if val, exists := c.Get("chatBot"); exists {
		chatBot = val.(*ChatBot)
	} else {
		logger.Printf("ERROR: failed to retrieve ChatBot from context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve ChatBot from context"})
		return
	}
	bearerToken, err := c.Cookie("BearerToken")
	if err == nil {
		chatBot.config.ApiKey = bearerToken
	} else {
		logger.Printf("WARNING: BearerToken is not found in cookies content")
		if len(chatBot.config.ApiKey) == 0 {
			logger.Printf("ERROR: ApiKey is not defined")
			c.JSON(http.StatusUnauthorized, gin.H{"error": "BearerToken is not found in cookies content"})
			return
		}
	}
	c.HTML(http.StatusOK, "index.html", gin.H{
		"version": chatBot.version,
		"config": chatBot.config,
	})
}

func keepalive(conn *websocket.Conn) {
	logger := log.New(gin.DefaultWriter, "[CHATBOT] ", log.LstdFlags)
	ticker := time.NewTicker(60 * time.Second)
	defer func() {
		ticker.Stop()
	}()
	for {
		select {
		case <-ticker.C:
			if err := conn.WriteMessage(websocket.TextMessage, []byte(EventPing + ":ping")); err != nil {
				logger.Printf("ERROR: failed to send PING message: %s", err)
				return
			}
		}
	}
}

func handleWebSocket(c *gin.Context) {
	var chatBot *ChatBot
	logger := log.New(gin.DefaultWriter, "[CHATBOT] ", log.LstdFlags)
	if val, exists := c.Get("chatBot"); exists {
		chatBot = val.(*ChatBot)
	} else {
		logger.Printf("ERROR: failed to retrieve ChatBot from context")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve ChatBot from context"})
		return
	}
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.Printf("ERROR: failed to upgrade connection to websocket: %s", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upgrade connection to websocket: " + err.Error()})
		return
	}
	go keepalive(conn)
	bearerToken, err := c.Cookie("BearerToken")
	if err == nil {
		chatBot.config.ApiKey = bearerToken
	} else {
		logger.Printf("WARNING: BearerToken is not found in cookies content")
		if len(chatBot.config.ApiKey) == 0 {
			logger.Printf("ERROR: ApiKey is not defined")
			conn.WriteMessage(websocket.TextMessage, []byte(EventDiagnostic + `:<p style="color: red;"><strong>Websocket error: </strong>BearerToken is not found in cookies content</p>`))
		}
	}
	defer conn.Close()
	resetHistory := true
	var responseChan chan string
	var cancel context.CancelFunc
	for {
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				logger.Printf("ERROR: failure while reading websocket message: %s", err.Error())
			} else {
				logger.Printf("INFO: websocket connection closed")
			}
			return
		}
		switch messageType {
		case websocket.CloseMessage:
			logger.Printf("INFO: peer initiated connection close, closing websocket")
			break
		case websocket.TextMessage, websocket.BinaryMessage:
			var err error
			re := regexp.MustCompile(`^(\d+):`)
			subMatch := re.FindStringSubmatch(string(message))
			if len(subMatch) != 2 {
				logger.Printf("ERROR: received unrecognized websocket message: %s", string(message))
				conn.WriteMessage(messageType, []byte(EventDiagnostic + `:<p style="color: red;"><strong>Websocket error: </strong>` + fmt.Sprintf("received unrecognized websocket message: \"%s\"", string(message)) + `</p>`))
				return
			}
			session := sessions.Default(c)
			switch subMatch[1] {
			case EventPing:
				logger.Printf("INFO: received EventPing")
				if err := conn.WriteMessage(messageType, []byte(EventPong + ":pong")); err != nil {
					logger.Printf("ERROR: failed to send PONG message: %s", err)
					return
				}
			case EventPong:
				logger.Printf("INFO: received PONG reply")
			case EventUserPrompt:
				// Process user prompt
				logger.Printf("INFO: received EventUserPrompt")
				go func() {
					responseChan, cancel = chatBot.startCompletionStream(session, string(message[3:]), resetHistory)
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
					cancel = nil
				}()
				err = conn.WriteMessage(messageType, []byte(EventConfirmed + ":" + EventUserPrompt))
			case EventCancelUserPrompt:
				if cancel != nil {
					logger.Printf("INFO: received EventCancelUserPrompt")
					cancel()
					err = conn.WriteMessage(messageType, []byte(EventConfirmed + ":" + EventCancelUserPrompt))
					messages := []openai.ChatCompletionMessageParamUnion{}
					session.Set("messages", messages)
					session.Save()
					resetHistory = true
				}
			case EventSystemPrompt:
				// Save system prompt
				logger.Printf("INFO: received EventSystemPrompt")
				session.Set("systemPrompt", string(message[3:]))
				messages := []openai.ChatCompletionMessageParamUnion{}
				session.Set("messages", messages)
				session.Save()
				err = conn.WriteMessage(messageType, []byte(EventConfirmed + ":" + EventSystemPrompt))
			case EventResetHistory:
				// Reset history
				if !resetHistory {
					logger.Printf("INFO: received EventResetHistory")
					if err = conn.WriteMessage(messageType, []byte(EventConfirmed + ":" + EventResetHistory)); err == nil {
						messages := []openai.ChatCompletionMessageParamUnion{}
						session.Set("messages", messages)
						session.Save()
						resetHistory = true
					}
				}
			case EventDisableHistory:
				// Disable history
				logger.Printf("INFO: received EventDisableHistory")
				chatBot.config.ChatOptions.ChatHistory = false
				err = conn.WriteMessage(messageType, []byte(EventConfirmed + ":" + EventDisableHistory))
			case EventEnableHistory:
				// Enable history
				logger.Printf("INFO: received EventEnableHistory")
				chatBot.config.ChatOptions.ChatHistory = true
				err = conn.WriteMessage(messageType, []byte(EventConfirmed + ":" + EventEnableHistory))
			case EventLoadSystemPrompt:
				// Load system prompt
				logger.Printf("INFO: received EventLoadSystemPrompt")
				err = conn.WriteMessage(messageType, []byte(EventLoadSystemPrompt + ":" + chatBot.config.SystemPrompt))
			default:
				logger.Printf("ERROR: received unrecognized websocket event: %s", string(message))
				conn.WriteMessage(messageType, []byte(EventDiagnostic + `:<p style="color: red;"><strong>Websocket error: </strong>` + fmt.Sprintf("received unrecognized websocket event: \"%s\"", string(message)) + `</p>`))
			}
			if err != nil {
				logger.Printf("ERROR: websocket error: %s", err)
			}
		case websocket.PingMessage:
			logger.Printf("INFO: received system PING message", err)
		case websocket.PongMessage:
			logger.Printf("INFO: received system PONG message", err)
		default:
			logger.Printf("INFO: received unrecognized message type %d", messageType)
		}
	}
}
