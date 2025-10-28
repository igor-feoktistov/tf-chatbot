package main

import (
	"log"
	"flag"

	"tf-chatbot/internal/chatbot"
)

func main() {
	var chatBot *chatbot.ChatBot
	var err error
        optConfig := flag.String("configPath", "/usr/local/etc/chatbotConfig.yaml", "ChatBot configuration path")
        flag.Parse()
        if len(*optConfig) == 0 {
                log.Fatal("expected ChatBot configuration path argument")
        }
        if chatBot, err = chatbot.ChatBotInitialize(*optConfig); err != nil {
		log.Fatalf("failure to initialize ChatBot: %s", err)
	}
	chatBot.Run()
}
