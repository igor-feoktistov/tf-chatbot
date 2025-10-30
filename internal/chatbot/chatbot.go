package chatbot

import (
	"fmt"
	"time"
	"log"
	"context"
	"io/ioutil"
	"strings"
	"encoding/json"
	"encoding/base64"

	"gopkg.in/yaml.v3"
	"github.com/gin-gonic/gin"
	"github.com/gin-contrib/multitemplate"
	"github.com/gin-contrib/sessions"
        "github.com/gin-contrib/sessions/cookie"
        "github.com/openai/openai-go/v3"
        "github.com/openai/openai-go/v3/option"

	"tf-chatbot/internal/utils"
)

const (
	LLM_STREAM_TIMEOUT = 300
)

type mcpServer struct {
	IntegrationFqn string `yaml:"integrationFqn" json:"integration_fqn"`
	EnableAllTools bool   `yaml:"enableAllTools" json:"enable_all_tools"`
	Tools []struct {
		Name string   `yaml:"name"           json:"name"`
	}                     `yaml:"tools"          json:"tools"`
}

type chatOptions struct {
	ChatHistory bool `yaml:"chatHistory" json:"chatHistory"`
}

type chatBotConfig struct {
	BaseUrl      string      `yaml:"baseUrl"      json:"baseUrl"`
	SessionKey   string      `yaml:"sessionKey"   json:"sessionKey"`
	ApiKey       string      `yaml:"apiKey"       json:"apiKey,omitempty"`
	SystemPrompt string      `yaml:"systemPrompt" json:"systemPrompt"`
	Model        string      `yaml:"model"        json:"model"`
	ChatOptions  chatOptions `yaml:"chatOptions"  json:"chatOptions,omitempty"`
	McpServers   []mcpServer `yaml:"mcpServers"   json:"mcpServers,omitempty"`
}

type Content struct {
	Content []ToolContent
}

type ToolContent struct {
	Type string
	Text string
}

type ChatBot struct {
        config     *chatBotConfig
        staticPath string
}

func ChatBotInitialize(configPath string, staticPath string) (chatBot *ChatBot, err error) {
	var b []byte
	if b, err = ioutil.ReadFile(configPath); err != nil {
		err = fmt.Errorf("Failed to read TrueFoundry configuration: ReadFile() failure: %s", err)
		return
	}
	config := &chatBotConfig{}
	if err = yaml.Unmarshal(b, config); err != nil {
		err = fmt.Errorf("Failed to parse ChatBot configuration: Unmarshal() failure: %s", err)
		return
	}
	if len(config.SystemPrompt) > 0 {
		if b, err = base64.StdEncoding.DecodeString(config.SystemPrompt); err != nil {
			err = fmt.Errorf("Error decoding default SystemPrompt: %s", err)
			return
		}
		config.SystemPrompt = string(b)
	}
	chatBot = &ChatBot{
		config: config,
		staticPath: staticPath,
	}
	return 
}

func (chatBot *ChatBot) Run() {
	sessionStore := cookie.NewStore([]byte(chatBot.config.SessionKey))
	sessionStore.Options(sessions.Options{MaxAge: 60 * 60 * 12})
	router := gin.Default()
	renderer := multitemplate.NewRenderer()
	router.Use(sessions.Sessions("aichat_session", sessionStore))
	router.Use(func(c *gin.Context) {
		c.Set("chatBot", chatBot)
		c.Next()
	})
	renderer.AddFromString("index.html", indexHTML)
	renderer.AddFromString("javascript.js", javascriptJS)
	router.HTMLRender = renderer
	router.Static("/static", chatBot.staticPath)
	router.GET("/", handleIndex)
	router.GET("javascript.js", handleJavascript)
	router.GET("/ws", handleWebSocket)
	if err := router.Run(":8080"); err != nil {
		log.Fatalf("Failed to run server: %v", err)
	}
}

func (chatBot *ChatBot) startCompletionStream(session sessions.Session, userPrompt string) chan string {
	responseChan := make(chan string)
	var systemPrompt string
	var messages []openai.ChatCompletionMessageParamUnion
	if v := session.Get("systemPrompt"); v != nil {
		systemPrompt = v.(string)
	}
	if len(systemPrompt) == 0 {
		systemPrompt = chatBot.config.SystemPrompt
	}
	openaiClient := openai.NewClient(
		option.WithBaseURL(chatBot.config.BaseUrl),
		option.WithAPIKey(chatBot.config.ApiKey),
		option.WithHeader("X-TFY-LOGGING-CONFIG", `{"enabled": true}`),
	)
	openaiParams := openai.ChatCompletionNewParams{
		Model: chatBot.config.Model,
		Temperature: openai.Float(0.7),
		MaxTokens:   openai.Int(4096),
	}
	if len(chatBot.config.McpServers) > 0 {
		openaiParams.SetExtraFields(map[string]any{
			"mcp_servers": chatBot.config.McpServers,
			"iteration_limit": 20,
		})
	}
	if chatBot.config.ChatOptions.ChatHistory {
		if v := session.Get("messages"); v != nil {
			messages = v.([]openai.ChatCompletionMessageParamUnion)
		}
	}
	if len(messages) > 0 {
		messages = append(messages, openai.UserMessage(userPrompt))
	} else {
		messages = []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(systemPrompt),
			openai.UserMessage(userPrompt),
		}
	}
	openaiParams.Messages = messages
	go func() {
		defer close(responseChan)
		ctx, cancel := context.WithTimeout(context.Background(), LLM_STREAM_TIMEOUT * time.Second)
		defer cancel()
		stream := openaiClient.Chat.Completions.NewStreaming(ctx, openaiParams)
		defer stream.Close()
		acc := openai.ChatCompletionAccumulator{}
		var assistantResponse []string
		var completeResponse []string
		for stream.Next() {
			chunk := stream.Current()
			acc.AddChunk(chunk)
			if _, ok := acc.JustFinishedToolCall(); ok {
				continue
                	}
                	if _, ok := acc.JustFinishedContent(); ok {
                        	continue
			}
			if _, ok := acc.JustFinishedRefusal(); ok {
                        	continue
			}
			if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
				completeResponse = append(completeResponse, chunk.Choices[0].Delta.Content)
				if strings.HasPrefix(chunk.Choices[0].Delta.Content, "{") ||
					strings.HasPrefix(chunk.Choices[0].Delta.Content, "undefined: {") ||
					strings.Contains(chunk.Choices[0].Delta.Content, `map[command:`) {
					if len(assistantResponse) > 0 {
						responseChan <- string(utils.MDtoHTML([]byte(strings.Join(assistantResponse, ""))))
						assistantResponse = []string{}
					}
					content := Content{}
					if err := json.Unmarshal([]byte(chunk.Choices[0].Delta.Content), &content); err == nil {
						if len(content.Content) > 0 {
							responseChan <- fmt.Sprintf(`<details class="chat"><summary class="chat">tool call</summary><div class="chat"><textarea class="chat" id="toolCall" rows="12" cols="128">%s</textarea></div></details>`, content.Content[0].Text)
						}
					}
					continue
				}
				assistantResponse = append(assistantResponse, chunk.Choices[0].Delta.Content)
			}
		}
		if len(assistantResponse) > 0 {
			responseChan <- string(utils.MDtoHTML([]byte(strings.Join(assistantResponse, ""))))
		}
		if err := stream.Err(); err == nil {
			if acc.Usage.TotalTokens >  0 {
				log.Printf("INFO: finished completion streaming, total tokens: %d", acc.Usage.TotalTokens)
			}
			// Preserve last conversation history
			if chatBot.config.ChatOptions.ChatHistory {
				messages = []openai.ChatCompletionMessageParamUnion{
					openai.SystemMessage(systemPrompt),
					openai.UserMessage(userPrompt),
					openai.AssistantMessage(strings.Join(completeResponse, "")),
				}
				session.Set("messages", messages)
				session.Save()
			}
		} else {
			log.Printf("ERROR: LLM stream response error: %s", err)
			responseChan <- `<p style="color: red;"><strong>LLM stream response error: </strong>` + err.Error() + `</p>`
		}
	}()
	return responseChan
}
